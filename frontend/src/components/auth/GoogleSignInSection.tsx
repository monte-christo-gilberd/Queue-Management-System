/**
 * GoogleSignInSection: pemisah "atau" dan tombol Google yang sudah tersambung ke login.
 *
 * Dipakai oleh : components/LoginScreen.tsx, components/RegisterScreen.tsx
 * Bergantung   : hooks/use-google-sign-in, store/authStore (loginWithGoogle), lib/auth-errors (describeGoogleError),
 *                lib/google-auth (isGoogleEnabled), components/ui/Toast, expo-router
 * Peran        : publik
 * Catatan      : Tidak merender apa pun (termasuk pemisah "atau") bila platform yang berjalan tidak punya client ID Google.
 *                Di web pada origin yang tidak aman (`http://<alamat-IP>`), tombol diganti keterangan: Google hanya menerima
 *                `https`/`localhost` dan WebCrypto tidak tersedia, jadi menekan tombol hanya memunculkan galat.
 *                Setelah login: ke `next` yang aman atau /dashboard. Email sudah terdaftar dengan password: ke
 *                /link-google (konfirmasi password), membawa `next`.
 */
import { router, type Href } from 'expo-router';
import { useCallback, useState } from 'react';
import { Platform, Text } from 'react-native';

import { GoogleButton, OrDivider } from '@/components/auth/AuthButtons';
import { useToast } from '@/components/ui/Toast';
import { useGoogleSignIn } from '@/hooks/use-google-sign-in';
import { describeGoogleError } from '@/lib/auth-errors';
import { isGoogleEnabled, isSecureOrigin } from '@/lib/google-auth';
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
  // Origin tidak aman: tombol pasti gagal. Jelaskan alasannya, jangan biarkan pengguna menemui galat mentah.
  if (!isSecureOrigin(Platform.OS)) {
    return (
      <>
        <OrDivider />
        <Text accessibilityRole="alert" className="text-center text-[12px] leading-5 text-placeholder">
          Login dengan Google hanya tersedia lewat https atau http://localhost. Buka aplikasi lewat alamat itu.
        </Text>
      </>
    );
  }
  return <GoogleSignInEnabled {...props} />;
}

/** Bagian dalam: memanggil hook Google. Hanya dirender bila client ID platform ada. */
function GoogleSignInEnabled({ label, next }: GoogleSignInSectionProps) {
  // Aksi login Google dari store dan fungsi toast untuk pesan galat.
  const loginWithGoogle = useAuthStore((s) => s.loginWithGoogle);
  const { show } = useToast();
  const [loading, setLoading] = useState(false);

  // Dipanggil hook Google dengan ID token setelah pengguna memilih akun.
  const onIdToken = useCallback(
    async (idToken: string) => {
      // Kunci tombol selama login berjalan.
      setLoading(true);
      try {
        // Server menukar ID token dengan sesi, atau meminta penautan akun.
        const result = await loginWithGoogle(idToken);
        if (result === 'link_required') {
          // Email sudah terdaftar dengan password: minta konfirmasi password sebelum menautkan.
          router.push((next ? `/link-google?next=${encodeURIComponent(next)}` : '/link-google') as Href);
        } else {
          // replace (bukan push) agar tombol kembali tidak membawa pengguna ke halaman login lagi.
          router.replace((next || DEFAULT_AFTER_LOGIN) as '/dashboard');
        }
      } catch (e) {
        // Galat API diubah ke pesan ramah lewat toast.
        show(describeGoogleError(e));
      } finally {
        setLoading(false);
      }
    },
    [loginWithGoogle, next, show],
  );

  // ready: permintaan auth Google siap dipakai. signIn: membuka layar pilih akun Google.
  const { ready, signIn } = useGoogleSignIn((t) => void onIdToken(t), show);

  // Membuka layar Google bisa melempar (mis. WebCrypto/popup diblokir). Tangkap supaya jadi toast, bukan layar galat.
  const handlePress = useCallback(async () => {
    try {
      await signIn();
    } catch {
      show('Login dengan Google gagal dibuka, coba lagi');
    }
  }, [signIn, show]);

  // Tombol dikunci sampai permintaan siap atau saat login sedang berjalan.
  return (
    <>
      <OrDivider />
      <GoogleButton label={label} onPress={() => void handlePress()} disabled={!ready || loading} />
    </>
  );
}
