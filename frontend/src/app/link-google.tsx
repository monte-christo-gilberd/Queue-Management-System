/**
 * link-google: konfirmasi password untuk menautkan akun Google ke akun yang sudah terdaftar dengan email yang sama.
 *
 * Rute        : /link-google (query next opsional).
 * Akses       : guest-only (AuthGate mode guest).
 * Dipakai oleh: GoogleSignInSection (setelah 409 GOOGLE_LINK_REQUIRED dari login Google).
 * Bergantung  : AuthGate, LinkGoogleScreen, useToast, useAuthStore (pendingGoogleLink, linkGoogle, clearPendingGoogleLink),
 *               describeLinkError, isLinkSessionOver, sanitizeNext.
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
  // Aksi dan status penautan dari store: link token yang menunggu, pembersihnya, dan token login.
  const linkGoogle = useAuthStore((s) => s.linkGoogle);
  const pending = useAuthStore((s) => s.pendingGoogleLink);
  const clearPending = useAuthStore((s) => s.clearPendingGoogleLink);
  const token = useAuthStore((s) => s.token);
  // Toast global, dipakai untuk pesan yang harus tetap tampil setelah layar dilepas.
  const { show } = useToast();
  const { next } = useLocalSearchParams<{ next?: string }>();
  // Sanitasi next agar tujuan setelah menaut tidak bisa mengarah ke luar aplikasi.
  const safeNext = sanitizeNext(next, '');
  // State form dan status permintaan.
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  // Tanpa link token yang menunggu tidak ada yang bisa dikonfirmasi. Sudah login (token terisi) ditangani AuthGate.
  useEffect(() => {
    // replace agar halaman tanpa link token tidak tersisa di riwayat.
    if (!pending && !token) router.replace('/login');
  }, [pending, token]);

  /** Kirim password untuk menautkan akun Google; berhasil pindah ke tujuan, gagal tampilkan pesan. */
  async function handleSubmit() {
    // Tolak password kosong sebelum memanggil API.
    if (!password) {
      setErrorMessage('Password wajib diisi');
      return;
    }
    // Mulai loading dan hapus galat lama.
    setLoading(true);
    setErrorMessage(undefined);
    try {
      // Store membawa link token dari login Google bersama password sebagai bukti kepemilikan akun.
      await linkGoogle(password);
      // replace agar layar penautan tidak tersisa di riwayat.
      router.replace((safeNext || DEFAULT_AFTER_LOGIN) as '/dashboard');
    } catch (e) {
      // Sesi penautan yang sudah tidak bisa dipakai dibedakan dari salah password biasa.
      if (isLinkSessionOver(e)) {
        // Sesi penautan berakhir: store sudah mengosongkan link token, jadi efek di atas segera mengalihkan ke login dan
        // layar ini dilepas. Pesan harus lewat provider global (bukan state layar), kalau tidak tidak pernah tampil.
        show(describeLinkError(e));
        setPassword('');
      } else {
        // Salah password atau galat lain: pesan tampil di layar, pengguna boleh mencoba lagi.
        setErrorMessage(describeLinkError(e));
      }
    } finally {
      // Selalu lepas kunci tombol.
      setLoading(false);
    }
  }

  /** Batalkan penautan: buang link token yang menunggu lalu kembali ke login. */
  function handleCancel() {
    // Buang link token agar tidak tertinggal di memori.
    clearPending();
    // replace agar layar penautan tidak bisa dibuka lagi lewat tombol kembali.
    router.replace('/login');
  }

  // Guest-only: pengguna yang sudah login tidak perlu menaut akun.
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
