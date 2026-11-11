/**
 * forgot-password: form lupa password: meminta tautan reset dikirim ke email.
 *
 * Rute        : /forgot-password.
 * Akses       : publik (tanpa AuthGate).
 * Dipakai oleh: tautan Forgot password di LoginScreen dan tombol Minta Tautan Baru di ResetPasswordScreen.
 * Bergantung  : ForgotPasswordScreen, requestPasswordReset dan describeForgotError (lib/password-reset).
 * Catatan     : Tautan reset yang dikirim membuka /reset-password?token=... Pesan sukses selalu sama, apakah email terdaftar atau tidak.
 */
import { useState } from 'react';

import ForgotPasswordScreen from '@/components/ForgotPasswordScreen';
import { describeForgotError, requestPasswordReset } from '@/lib/password-reset';

/** Pesan sukses tetap, dibuat generik agar tidak membocorkan apakah sebuah email punya akun. */
const SENT_MESSAGE =
  'Jika email itu terdaftar, tautan untuk membuat password baru sudah kami kirim. Periksa kotak masuk (dan folder spam) Anda.';

/**
 * Form lupa password. Pesan sukses sengaja sama untuk email yang terdaftar maupun tidak.
 */
export default function ForgotPasswordRoute() {
  // State form, status permintaan, dan penanda permintaan sudah diterima.
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  const [sent, setSent] = useState(false);

  /** Validasi email, minta tautan reset, lalu tandai terkirim; galat ditampilkan lewat errorMessage. */
  async function handleSubmit() {
    // Email dipangkas supaya spasi tak sengaja tidak membuat permintaan gagal.
    const value = email.trim();
    // Tolak email kosong sebelum memanggil API.
    if (!value) {
      setErrorMessage('Email wajib diisi');
      return;
    }
    // Mulai loading dan hapus galat lama.
    setLoading(true);
    setErrorMessage(undefined);
    try {
      // Server menjawab sama untuk email terdaftar maupun tidak.
      await requestPasswordReset(value);
      setSent(true);
    } catch (e) {
      // Ubah galat API ke pesan ramah.
      setErrorMessage(describeForgotError(e));
    } finally {
      // Selalu lepas kunci tombol.
      setLoading(false);
    }
  }

  // Pesan konfirmasi hanya diberikan setelah permintaan diterima; sebelum itu layar menampilkan form.
  return (
    <ForgotPasswordScreen
      email={email}
      onChangeEmail={setEmail}
      onSubmit={() => void handleSubmit()}
      loading={loading}
      errorMessage={errorMessage}
      sentMessage={sent ? SENT_MESSAGE : undefined}
    />
  );
}
