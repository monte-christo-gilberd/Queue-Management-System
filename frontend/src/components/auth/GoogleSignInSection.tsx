/**
 * GoogleSignInSection: pemisah "atau" dan tombol Google yang sudah tersambung ke login.
 *
 * Dipakai oleh : components/LoginScreen.tsx, components/RegisterScreen.tsx
 * Bergantung   : hooks/use-google-sign-in, store/authStore (loginWithGoogle), lib/auth-errors (describeGoogleError),
 *                lib/google-auth (isGoogleEnabled), components/ui/Toast, expo-router
 * Peran        : publik
 * Catatan      : Tidak merender apa pun (termasuk pemisah "atau") bila platform yang berjalan tidak punya client ID Google.
 *                Setelah login: ke `next` yang aman atau /dashboard. Email sudah terdaftar dengan password: ke
 *                /link-google (konfirmasi password), membawa `next`.
 */
import { router, type Href } from 'expo-router';
import { useCallback, useState } from 'react';
import { Platform } from 'react-native';

import { GoogleButton, OrDivider } from '@/components/auth/AuthButtons';
import { useToast } from '@/components/ui/Toast';
import { useGoogleSignIn } from '@/hooks/use-google-sign-in';
import { describeGoogleError } from '@/lib/auth-errors';
import { isGoogleEnabled } from '@/lib/google-auth';
import { DEFAULT_AFTER_LOGIN } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';

/** Props bagian Google. */
export type GoogleSignInSectionProps = {
  /** Teks tombol. Default "Login with Google". */
  label?: string;
  /** Tujuan setelah berhasil; sudah disanitasi oleh route (sanitizeNext). Kosong = /dashboard. */
  next?: string;
};

/** Pemisah "atau" dan tombol Google; tidak tampil bila Google belum dikonfigurasi untuk platform ini. */
export function GoogleSignInSection(props: GoogleSignInSectionProps) {
  // Hook provider melempar galat tanpa client ID, jadi hook-nya hanya hidup di komponen dalam yang dirender bila aktif.
  if (!isGoogleEnabled(Platform.OS)) return null;
  return <GoogleSignInEnabled {...props} />;
}

/** Bagian dalam: memanggil hook Google. Hanya dirender bila client ID platform ada. */
function GoogleSignInEnabled({ label, next }: GoogleSignInSectionProps) {
  const loginWithGoogle = useAuthStore((s) => s.loginWithGoogle);
  const { show } = useToast();
  const [loading, setLoading] = useState(false);

  const onIdToken = useCallback(
    async (idToken: string) => {
      setLoading(true);
      try {
        const result = await loginWithGoogle(idToken);
        if (result === 'link_required') {
          // Email sudah terdaftar dengan password: minta konfirmasi password sebelum menautkan.
          router.push((next ? `/link-google?next=${encodeURIComponent(next)}` : '/link-google') as Href);
        } else {
          router.replace((next || DEFAULT_AFTER_LOGIN) as '/dashboard');
        }
      } catch (e) {
        show(describeGoogleError(e));
      } finally {
        setLoading(false);
      }
    },
    [loginWithGoogle, next, show],
  );

  const { ready, signIn } = useGoogleSignIn((t) => void onIdToken(t), show);

  return (
    <>
      <OrDivider />
      <GoogleButton label={label} onPress={() => void signIn()} disabled={!ready || loading} />
    </>
  );
}
