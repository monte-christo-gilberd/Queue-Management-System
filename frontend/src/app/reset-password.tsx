/**
 * reset-password: form password baru dari tautan di email reset (/reset-password?token=...).
 *
 * Rute        : /reset-password (query token wajib).
 * Akses       : publik (tanpa AuthGate), karena pengguna lupa password tidak punya sesi.
 * Dipakai oleh: tautan di email yang dikirim setelah lupa password (forgot-password.tsx).
 * Bergantung  : ResetPasswordScreen, resetPassword/isInvalidResetToken/describeResetError (lib/password-reset), useAuthStore (logout),
 *               validatePassword, getConfirmPasswordError (utils/confirm-password).
 * Catatan     : Tanpa token, halaman langsung masuk mode invalid. Setelah berhasil, sesi lokal dihapus agar halaman login tampil.
 *               Pesan "Password tidak sama" tampil inline setelah field konfirmasi kehilangan fokus atau Simpan Password
 *               ditekan, bukan sambil mengetik.
 */
import { useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import ResetPasswordScreen from '@/components/ResetPasswordScreen';
import { describeResetError, isInvalidResetToken, resetPassword } from '@/lib/password-reset';
import { useAuthStore } from '@/store/authStore';
import { getConfirmPasswordError } from '@/utils/confirm-password';
import { validatePassword } from '@/utils/password';

/**
 * Halaman publik (tanpa AuthGate): orang yang lupa password tidak punya sesi, dan yang sedang masuk pun boleh
 * membukanya. Tautan datang dari email: /reset-password?token=...
 */
export default function ResetPasswordRoute() {
  const { token } = useLocalSearchParams<{ token?: string | string[] }>();
  const rawToken = (Array.isArray(token) ? token[0] : token) ?? '';
  const logout = useAuthStore((s) => s.logout);
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  // true setelah field konfirmasi kehilangan fokus atau Simpan Password ditekan; sebelum itu pesan "tidak sama" disembunyikan.
  const [confirmTouched, setConfirmTouched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  const [mode, setMode] = useState<'form' | 'invalid' | 'done'>(rawToken ? 'form' : 'invalid');
  // Pesan inline di bawah field konfirmasi. Pesan hilang sendiri saat isinya sudah sama dengan password.
  const confirmPasswordError = getConfirmPasswordError(password, confirmPassword, confirmTouched);

  async function handleSubmit() {
    // Setelah Simpan Password ditekan, ketidakcocokan boleh ditampilkan walau fokus belum meninggalkan field konfirmasi.
    setConfirmTouched(true);
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
      await resetPassword(rawToken, password);
      // Sesi lama sudah dicabut server; bersihkan juga yang tersimpan di perangkat ini agar halaman login tampil.
      logout();
      setMode('done');
    } catch (e) {
      if (isInvalidResetToken(e)) setMode('invalid');
      else setErrorMessage(describeResetError(e));
    } finally {
      setLoading(false);
    }
  }

  return (
    <ResetPasswordScreen
      mode={mode}
      password={password}
      confirmPassword={confirmPassword}
      onChangePassword={setPassword}
      onChangeConfirmPassword={setConfirmPassword}
      onBlurConfirmPassword={() => setConfirmTouched(true)}
      confirmPasswordError={confirmPasswordError}
      onSubmit={() => void handleSubmit()}
      loading={loading}
      errorMessage={errorMessage}
    />
  );
}
