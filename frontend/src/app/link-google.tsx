/**
 * link-google: konfirmasi password untuk menautkan akun Google ke akun yang sudah terdaftar dengan email yang sama.
 *
 * Rute        : /link-google (query next opsional).
 * Akses       : guest-only (AuthGate mode guest).
 * Dipakai oleh: GoogleSignInSection (setelah 409 GOOGLE_LINK_REQUIRED dari login Google).
 * Bergantung  : LinkGoogleScreen, useAuthStore (pendingGoogleLink, linkGoogle), describeLinkError, sanitizeNext.
 * Catatan     : Link token hanya di memori (hilang saat refresh) dan hanya hidup 5 menit. Tanpa link token yang menunggu
 *               (mis. halaman dibuka langsung atau di-refresh), pengguna dikembalikan ke halaman login. Link token yang
 *               tidak bisa dipakai lagi (habis, terlalu sering salah) juga mengembalikan ke login, DENGAN pesan: toast
 *               ditampilkan lewat useToast (provider ada di layout root) karena layar ini ikut dilepas saat dialihkan,
 *               jadi state pesan galat milik layar tidak sempat tampil.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import { useToast } from '@/components/ui/Toast';
import LinkGoogleScreen from '@/components/LinkGoogleScreen';
import { describeLinkError } from '@/lib/auth-errors';
import { isLinkSessionOver } from '@/lib/google-login';
import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';

/** Layar konfirmasi password: mengirim password ke auth store, lalu pindah ke tujuan setelah berhasil. */
export default function LinkGoogleRoute() {
  const linkGoogle = useAuthStore((s) => s.linkGoogle);
  const pending = useAuthStore((s) => s.pendingGoogleLink);
  const clearPending = useAuthStore((s) => s.clearPendingGoogleLink);
  const token = useAuthStore((s) => s.token);
  const { show } = useToast();
  const { next } = useLocalSearchParams<{ next?: string }>();
  const safeNext = sanitizeNext(next, '');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  // Tanpa link token yang menunggu tidak ada yang bisa dikonfirmasi. Sudah login (token terisi) ditangani AuthGate.
  useEffect(() => {
    if (!pending && !token) router.replace('/');
  }, [pending, token]);

  async function handleSubmit() {
    if (!password) {
      setErrorMessage('Password wajib diisi');
      return;
    }
    setLoading(true);
    setErrorMessage(undefined);
    try {
      await linkGoogle(password);
      router.replace((safeNext || DEFAULT_AFTER_LOGIN) as '/dashboard');
    } catch (e) {
      if (isLinkSessionOver(e)) {
        // Sesi penautan berakhir: store sudah mengosongkan link token, jadi efek di atas segera mengalihkan ke login dan
        // layar ini dilepas. Pesan harus lewat provider global (bukan state layar), kalau tidak tidak pernah tampil.
        show(describeLinkError(e));
        setPassword('');
      } else {
        setErrorMessage(describeLinkError(e));
      }
    } finally {
      setLoading(false);
    }
  }

  function handleCancel() {
    clearPending();
    router.replace('/');
  }

  return (
    <AuthGate mode="guest">
      <LinkGoogleScreen
        password={password}
        onChangePassword={setPassword}
        onSubmit={() => void handleSubmit()}
        onCancel={handleCancel}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
