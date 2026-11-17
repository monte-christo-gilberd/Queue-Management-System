/**
 * authStore.ts: store autentikasi (token dan nama tampilan) yang dipersist ke storage.
 *
 * Dipakai oleh : components/auth/AuthGate.tsx, app/login.tsx, app/register.tsx, hooks/use-auth-hydrated.ts,
 *                dan layar di grup (user).
 * Bergantung   : lib/api.ts (apiFetch, setAuthToken, setOnUnauthorized), store/avatarStore.ts, store/safe-storage.ts.
 * Alur         : login/register -> set token -> setAuthToken (klien API). 401 dari API -> logout() ->
 *                AuthGate mengarahkan ke login. Login Google: loginWithGoogle -> token, atau `link_required` (email sudah
 *                terdaftar dengan password) -> pendingGoogleLink -> layar /link-google -> linkGoogle(password).
 * Catatan      : key storage `qms-auth`; hanya token dan displayName yang dipersist. `pendingGoogleLink` (link token
 *                5 menit) SENGAJA hanya di memori: tidak dipersist, hilang saat refresh. Lihat AUTH_FLOW_GUIDE.md.
 */
import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

import { apiFetch, setAuthToken, setOnUnauthorized } from '@/lib/api';
import { confirmGoogleLink, exchangeGoogleIdToken, isLinkSessionOver } from '@/lib/google-login';
import type { AuthResponse } from '@/types/api';

import { useAvatarStore } from './avatarStore';
import { createSafeStorage } from './safe-storage';

/**
 * loadUsername mengambil username dari profil untuk header (backend tidak mengirim nama saat login Google).
 * Dipanggil setelah token tersimpan. Gagal mengambil tidak boleh menggagalkan login, jadi mengembalikan null.
 */
async function loadUsername(): Promise<string | null> {
  try {
    const me = await apiFetch<{ username: string }>('/api/v1/users/me');
    return me.username || null;
  } catch {
    return null;
  }
}

/**
 * Status autentikasi. Disimpan di AsyncStorage (web: localStorage) sehingga login bertahan saat refresh.
 * Hanya `token` dan `displayName` yang dipersist; aksi tidak ikut disimpan.
 */
export type AuthState = {
  /** Token sesi dari backend; null bila belum login atau sudah logout. Dikirim ke API lewat setAuthToken. */
  token: string | null;
  /** Nama yang ditampilkan di header: username saat register, identifier saat login (backend tidak mengirim nama). */
  displayName: string | null;
  /** Login dengan email/username dan password; menyimpan token. Galat API dilempar ke pemanggil. */
  login: (identifier: string, password: string) => Promise<void>;
  /** Mendaftar akun baru; menyimpan token. Galat API (mis. CONFLICT) dilempar ke pemanggil. */
  register: (email: string, username: string, password: string) => Promise<void>;
  /** Link token dari 409 GOOGLE_LINK_REQUIRED, menunggu konfirmasi password. Hanya di memori; null bila tidak ada. */
  pendingGoogleLink: string | null;
  /**
   * Login dengan ID token Google. Mengembalikan `ok` (token tersimpan) atau `link_required` (email sudah terdaftar
   * dengan password: `pendingGoogleLink` terisi, arahkan ke /link-google). Galat API dilempar ke pemanggil.
   */
  loginWithGoogle: (idToken: string) => Promise<'ok' | 'link_required'>;
  /**
   * Menautkan akun Google setelah password dibuktikan; menyimpan token. Salah password dilempar (link token tetap
   * berlaku untuk percobaan berikutnya); bila link token tidak bisa dipakai lagi, `pendingGoogleLink` dikosongkan.
   */
  linkGoogle: (password: string) => Promise<void>;
  /** Membuang link token yang menunggu (pengguna membatalkan). */
  clearPendingGoogleLink: () => void;
  /** Mengosongkan token, nama tampilan, dan penanda foto profil. */
  logout: () => void;
  /** Dipakai setelah username diubah di profil, supaya header ikut berubah. */
  setDisplayName: (name: string) => void;
};

/**
 * useAuthStore: store zustand autentikasi dengan middleware persist (key `qms-auth`).
 */
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      displayName: null,
      pendingGoogleLink: null,

      login: async (identifier, password) => {
        const { token } = await apiFetch<AuthResponse>('/api/v1/users/login', {
          method: 'POST',
          body: { identifier, password },
        });
        set({ token, displayName: identifier });
      },

      register: async (email, username, password) => {
        const { token } = await apiFetch<AuthResponse>('/api/v1/users/register', {
          method: 'POST',
          body: { email, username, password },
        });
        set({ token, displayName: username });
      },

      loginWithGoogle: async (idToken) => {
        const result = await exchangeGoogleIdToken(idToken);
        if (result.kind === 'link_required') {
          set({ pendingGoogleLink: result.linkToken });
          return 'link_required';
        }
        set({ token: result.token, pendingGoogleLink: null });
        // Token sudah terpasang di klien API (subscribe di bawah): ambil username untuk header.
        set({ displayName: await loadUsername() });
        return 'ok';
      },

      linkGoogle: async (password) => {
        const linkToken = useAuthStore.getState().pendingGoogleLink;
        if (!linkToken) throw new Error('Tidak ada penautan Google yang menunggu');
        try {
          const token = await confirmGoogleLink(linkToken, password);
          set({ token, pendingGoogleLink: null });
          set({ displayName: await loadUsername() });
        } catch (e) {
          // Link token habis, terbakar, atau akun sudah tertaut: tidak bisa dicoba lagi.
          if (isLinkSessionOver(e)) set({ pendingGoogleLink: null });
          throw e;
        }
      },

      clearPendingGoogleLink: () => set({ pendingGoogleLink: null }),

      logout: () => {
        // Foto user sebelumnya tidak boleh sempat tampil untuk user berikutnya.
        useAvatarStore.getState().setUpdatedAt(null);
        set({ token: null, displayName: null, pendingGoogleLink: null });
      },

      setDisplayName: (name) => set({ displayName: name }),
    }),
    {
      name: 'qms-auth',
      // dibungkus agar kegagalan baca/tulis tidak membuat hydrate menggantung (lihat safe-storage.ts)
      storage: createJSONStorage(() => createSafeStorage(AsyncStorage)),
      partialize: (state) => ({ token: state.token, displayName: state.displayName }),
    },
  ),
);

// Klien API selalu sinkron dengan token di store, termasuk setelah rehydrate dari storage.
setAuthToken(useAuthStore.getState().token);
useAuthStore.subscribe((state) => setAuthToken(state.token));
// Token ditolak backend (401) berarti sesi habis: keluar. Redirect ke login dilakukan oleh AuthGate.
setOnUnauthorized(() => useAuthStore.getState().logout());
