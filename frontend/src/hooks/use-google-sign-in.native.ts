/**
 * use-google-sign-in.native.ts: login Google native (Android/iOS) lewat SDK Google Sign-In (Credential Manager di Android).
 *
 * Dipakai oleh : components/auth/GoogleSignInSection.tsx (lewat `@/hooks/use-google-sign-in`; Metro memilih file ini di native).
 * Bergantung   : @react-native-google-signin/google-signin, lib/google-auth.ts (client ID per platform).
 * Kenapa       : alur redirect expo-auth-session di Android memakai custom URI scheme (`<package>:/oauthredirect`), dan Google
 *                menolaknya untuk OAuth client bertipe Android (Error 400: invalid_request). SDK native tidak memakai redirect.
 * Catatan      : `webClientId` WAJIB: SDK memakai web client ID sebagai audience (`aud`) ID token, jadi backend
 *                (GOOGLE_CLIENT_IDS) harus memuat web client ID. Android client hanya perlu package name + SHA-1 yang cocok
 *                di Google Cloud. Butuh dev/production build (tidak jalan di Expo Go).
 */
import { GoogleSignin, isErrorWithCode, statusCodes } from '@react-native-google-signin/google-signin';
import { useCallback, useEffect, useRef } from 'react';

import { GOOGLE_CLIENT_IDS } from '@/lib/google-auth';

/** configureOnce: konfigurasi SDK sekali per proses. */
let configured = false;
function configureOnce() {
  if (configured) return;
  configured = true;
  GoogleSignin.configure({ webClientId: GOOGLE_CLIENT_IDS.web, scopes: ['email', 'profile'] });
}

/** useGoogleSignIn: mengembalikan `ready` dan `signIn` (buka pilih akun Google), sama seperti versi web. */
export function useGoogleSignIn(onIdToken: (idToken: string) => void, onError: (message: string) => void) {
  // Simpan callback di ref agar signIn stabil dan tidak perlu dibuat ulang tiap render.
  const onIdTokenRef = useRef(onIdToken);
  const onErrorRef = useRef(onError);
  useEffect(() => {
    onIdTokenRef.current = onIdToken;
    onErrorRef.current = onError;
  }, [onIdToken, onError]);

  useEffect(() => {
    configureOnce();
  }, []);

  const signIn = useCallback(async () => {
    try {
      // Pilih akun setiap kali (sama seperti selectAccount di versi web): putuskan sesi SDK lama dulu.
      await GoogleSignin.signOut().catch(() => undefined);
      const response = await GoogleSignin.signIn();
      // 'cancelled' = pengguna menutup pemilih akun: diam saja.
      if (response.type !== 'success') return;
      const idToken = response.data.idToken;
      if (idToken) onIdTokenRef.current(idToken);
      else onErrorRef.current('Login dengan Google gagal, coba lagi');
    } catch (e) {
      if (isErrorWithCode(e) && (e.code === statusCodes.SIGN_IN_CANCELLED || e.code === statusCodes.IN_PROGRESS)) return;
      onErrorRef.current('Login dengan Google gagal, coba lagi');
    }
  }, []);

  return { ready: true, signIn };
}
