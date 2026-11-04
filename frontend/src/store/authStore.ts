import AsyncStorage from '@react-native-async-storage/async-storage';
import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

import { apiFetch, setAuthToken, setOnUnauthorized } from '@/lib/api';
import type { AuthResponse } from '@/types/api';

import { useAvatarStore } from './avatarStore';
import { createSafeStorage } from './safe-storage';

/**
 * Status autentikasi. Disimpan di AsyncStorage (web: localStorage) sehingga login bertahan saat refresh.
 * Hanya `token` dan `displayName` yang dipersist; aksi tidak ikut disimpan.
 */
export type AuthState = {
  token: string | null;
  /** Nama yang ditampilkan di header: username saat register, identifier saat login (backend tidak mengirim nama). */
  displayName: string | null;
  login: (identifier: string, password: string) => Promise<void>;
  register: (email: string, username: string, password: string) => Promise<void>;
  logout: () => void;
  /** Dipakai setelah username diubah di profil, supaya header ikut berubah. */
  setDisplayName: (name: string) => void;
};

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      displayName: null,

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

      logout: () => {
        // Foto user sebelumnya tidak boleh sempat tampil untuk user berikutnya.
        useAvatarStore.getState().setUpdatedAt(null);
        set({ token: null, displayName: null });
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
