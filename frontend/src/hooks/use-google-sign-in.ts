/**
 * use-google-sign-in.ts: membuka layar masuk Google dan mengembalikan ID token ke pemanggil. Versi WEB (expo-auth-session).
 * Native (Android/iOS) memakai use-google-sign-in.native.ts (SDK Google Sign-In), karena redirect custom scheme ditolak Google.
 *
 * Dipakai oleh : components/auth/GoogleSignInSection.tsx.
 * Bergantung   : expo-auth-session (provider Google), expo-web-browser, lib/google-auth.ts (client ID per platform).
 * Alur         : signIn() -> Google -> response `success` dengan params.id_token -> onIdToken(idToken). Dibatalkan
 *                pengguna ('dismiss'/'cancel'/'locked'): diam saja. Galat lain -> onError(pesan).
 * Catatan      : HANYA boleh dipanggil bila isGoogleEnabled(Platform.OS): provider melempar galat bila client ID platform
 *                kosong (karena itu dibungkus komponen yang tidak merender hook ini tanpa client ID). Di native butuh dev
 *                build (expo run:android / expo run:ios), tidak jalan di Expo Go.
 */
import * as Google from 'expo-auth-session/providers/google';
import * as WebBrowser from 'expo-web-browser';
import { useEffect } from 'react';

import { GOOGLE_CLIENT_IDS } from '@/lib/google-auth';

// Wajib di web: menutup jendela popup login Google dan mengembalikan hasilnya ke jendela asal.
WebBrowser.maybeCompleteAuthSession();

/** useGoogleSignIn: mengembalikan `ready` (permintaan siap) dan `signIn` (buka layar Google). */
export function useGoogleSignIn(onIdToken: (idToken: string) => void, onError: (message: string) => void) {
  const [request, response, promptAsync] = Google.useIdTokenAuthRequest({
    webClientId: GOOGLE_CLIENT_IDS.web,
    androidClientId: GOOGLE_CLIENT_IDS.android,
    iosClientId: GOOGLE_CLIENT_IDS.ios,
    // Selalu tampilkan pemilih akun: pengguna dengan beberapa akun Google bisa memilih yang benar.
    selectAccount: true,
  });

  useEffect(() => {
    if (!response) return;
    if (response.type === 'success') {
      const idToken = response.params?.id_token;
      if (idToken) onIdToken(idToken);
      else onError('Login dengan Google gagal, coba lagi');
    } else if (response.type === 'error') {
      onError('Login dengan Google gagal, coba lagi');
    }
    // 'dismiss', 'cancel', dan 'locked' = pengguna menutup layar Google: tidak ada pesan.
    // onIdToken/onError sengaja tidak jadi dependensi: efek ini harus jalan sekali per response, bukan per render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [response]);

  return { ready: request !== null, signIn: () => promptAsync() };
}
