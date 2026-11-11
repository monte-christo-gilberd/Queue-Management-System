/**
 * register: form daftar akun baru untuk tamu; setelah berhasil diarahkan ke next yang aman atau ke dashboard.
 *
 * Rute        : /register (query next opsional).
 * Akses       : guest-only (AuthGate mode guest).
 * Dipakai oleh: tautan Daftar di halaman login (/), yang membawa next bila ada.
 * Bergantung  : RegisterScreen, useAuthStore (register), validatePassword (utils/password),
 *               getConfirmPasswordError (utils/confirm-password), sanitizeNext.
 * Catatan     : Validasi password dan konfirmasinya dilakukan di route, bukan di komponen tampilan. Pesan "Password tidak
 *               sama" tampil inline setelah field konfirmasi kehilangan fokus atau tombol Daftar ditekan, bukan sambil
 *               mengetik dan bukan sebagai toast. Galat saat kirim tetap tampil sebagai toast.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import RegisterScreen from '@/components/RegisterScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';
import { getConfirmPasswordError } from '@/utils/confirm-password';
import { validatePassword } from '@/utils/password';

/**
 * Form daftar: validasi isian lokal lalu mendaftar lewat auth store.
 */
export default function RegisterRoute() {
  const register = useAuthStore((s) => s.register);
  // next dari AuthGate atau tautan login (mis. dari /join): setelah daftar, kembali ke halaman itu, bukan dashboard.
  const { next } = useLocalSearchParams<{ next?: string }>();
  const safeNext = sanitizeNext(next, '');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  // true setelah field konfirmasi kehilangan fokus atau Daftar ditekan; sebelum itu pesan "tidak sama" disembunyikan.
  const [confirmTouched, setConfirmTouched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  // Pengecekan kesamaan password ada di route (bukan di komponen tampilan). Pesan hilang sendiri saat isinya sudah sama.
  const confirmPasswordError = getConfirmPasswordError(password, confirmPassword, confirmTouched);

  async function handleSubmit() {
    // Setelah Daftar ditekan, ketidakcocokan boleh ditampilkan walau fokus belum pernah meninggalkan field konfirmasi.
    setConfirmTouched(true);
    // Isian wajib kosong: hentikan sebelum validasi lain, pesan tampil sebagai toast.
    if (!username.trim() || !email.trim() || !password) {
      setErrorMessage('Username, email, dan password wajib diisi');
      return;
    }
    // Aturan password sama dengan backend; menolak lebih awal menghemat satu permintaan ke server.
    const weak = validatePassword(password);
    if (weak) {
      setErrorMessage(weak);
      return;
    }
    // Konfirmasi harus sama; pesan inline di bawah field juga tampil karena confirmTouched sudah true.
    if (password !== confirmPassword) {
      setErrorMessage('Password dan konfirmasi password harus sama');
      return;
    }
    // Isian lolos: mulai permintaan dan bersihkan toast galat lama.
    setLoading(true);
    setErrorMessage(undefined);
    try {
      // Buat akun lewat auth store; email dan username dipangkas spasinya, password dikirim apa adanya.
      await register(email.trim(), username.trim(), password);
      // Cast hanya agar tipe rute yang ketat menerima string dinamis dari ?next atau nilai default.
      router.replace((safeNext || DEFAULT_AFTER_LOGIN) as '/dashboard');
    } catch (e) {
      // Galat server atau jaringan diubah menjadi pesan ramah yang tampil sebagai toast.
      setErrorMessage(describeAuthError(e));
    } finally {
      // Apa pun hasilnya, lepaskan status loading agar tombol Daftar aktif lagi.
      setLoading(false);
    }
  }

  return (
    // Halaman hanya untuk tamu: AuthGate mode guest.
    <AuthGate mode="guest">
      <RegisterScreen
        username={username}
        email={email}
        password={password}
        confirmPassword={confirmPassword}
        onChangeUsername={setUsername}
        onChangeEmail={setEmail}
        onChangePassword={setPassword}
        onChangeConfirmPassword={setConfirmPassword}
        onBlurConfirmPassword={() => setConfirmTouched(true)}
        confirmPasswordError={confirmPasswordError}
        onSubmit={handleSubmit}
        next={safeNext}
        loginHref={safeNext ? `/?next=${encodeURIComponent(safeNext)}` : '/'}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
