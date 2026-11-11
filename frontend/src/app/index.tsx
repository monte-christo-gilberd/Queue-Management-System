/**
 * index: halaman login di "/", untuk tamu yang belum masuk.
 *
 * Rute        : / (menerima ?next=<path> dari AuthGate atau tautan register).
 * Akses       : guest-only (AuthGate mode guest); yang sudah punya token diarahkan ke tujuan next atau /dashboard.
 * Dipakai oleh: AuthGate di rute yang butuh login (mengirim ?next=), login.tsx (redirect dari /login), RegisterScreen dan
 *               link-google.tsx (kembali ke login).
 * Bergantung  : useAuthStore (login), AuthGate, LoginScreen, lib/safe-next, lib/auth-errors.
 * Catatan     : Tujuan setelah login: next yang sudah disanitasi, atau DEFAULT_AFTER_LOGIN (/dashboard) bila kosong.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import LoginScreen from '@/components/LoginScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';

/**
 * Halaman login. Mengirim identifier (email atau username) dan password ke auth store, lalu pindah ke tujuan setelah berhasil.
 */
export default function LoginRoute() {
  // Aksi login dari store.
  const login = useAuthStore((s) => s.login);
  // `next` dari AuthGate: halaman yang dituju sebelum diarahkan ke login.
  const { next } = useLocalSearchParams<{ next?: string }>();
  // Sanitasi next agar tidak bisa dipakai untuk membuka tujuan di luar aplikasi (open redirect).
  const safeNext = sanitizeNext(next, '');
  // State form dan status permintaan.
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  /** Validasi isian, kirim login, lalu pindah ke tujuan; galat ditampilkan lewat errorMessage. */
  async function handleSubmit() {
    // Tolak isian kosong sebelum memanggil API.
    if (!identifier.trim() || !password) {
      setErrorMessage('Email/username dan password wajib diisi');
      return;
    }
    // Mulai loading dan hapus galat lama.
    setLoading(true);
    setErrorMessage(undefined);
    try {
      // identifier dipangkas karena spasi tak sengaja sering ikut tersalin; password dikirim apa adanya.
      await login(identifier.trim(), password);
      // replace agar halaman login tidak tersisa di riwayat setelah masuk.
      // Cast hanya agar tipe rute yang ketat menerima string dinamis dari ?next atau nilai default.
      router.replace((safeNext || DEFAULT_AFTER_LOGIN) as '/dashboard');
    } catch (e) {
      // Ubah galat API ke pesan ramah.
      setErrorMessage(describeAuthError(e));
    } finally {
      // Selalu lepas kunci tombol, berhasil atau gagal.
      setLoading(false);
    }
  }

  // Guest-only: pengguna yang sudah punya token tidak perlu melihat form login.
  return (
    <AuthGate mode="guest">
      <LoginScreen
        identifier={identifier}
        password={password}
        onChangeIdentifier={setIdentifier}
        onChangePassword={setPassword}
        onSubmit={handleSubmit}
        next={safeNext}
        registerHref={safeNext ? `/register?next=${encodeURIComponent(safeNext)}` : '/register'}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
