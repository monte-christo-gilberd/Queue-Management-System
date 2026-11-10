/**
 * forgot-password: form lupa password: meminta tautan reset dikirim ke email.
 *
 * Rute        : /forgot-password.
 * Akses       : publik (tanpa AuthGate).
 * Dipakai oleh: tautan Lupa password di halaman login dan tombol ganti password di profil (lewat requestPasswordReset).
 * Bergantung  : ForgotPasswordScreen, requestPasswordReset dan describeForgotError (lib/password-reset).
 * Catatan     : Tautan reset yang dikirim membuka /reset-password?token=... Pesan sukses selalu sama, apakah email terdaftar atau tidak.
 */
import { useState } from 'react';

import ForgotPasswordScreen from '@/components/ForgotPasswordScreen';
import { describeForgotError, requestPasswordReset } from '@/lib/password-reset';

// Pesan dibuat generik agar tidak membocorkan apakah sebuah email punya akun.
const SENT_MESSAGE =
  'Jika email itu terdaftar, tautan untuk membuat password baru sudah kami kirim. Periksa kotak masuk (dan folder spam) Anda.';

/**
 * Form lupa password. Pesan sukses sengaja sama untuk email yang terdaftar maupun tidak.
 */
export default function ForgotPasswordRoute() {
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  const [sent, setSent] = useState(false);

  async function handleSubmit() {
    const value = email.trim();
    if (!value) {
      setErrorMessage('Email wajib diisi');
      return;
    }
    setLoading(true);
    setErrorMessage(undefined);
    try {
      await requestPasswordReset(value);
      setSent(true);
    } catch (e) {
      setErrorMessage(describeForgotError(e));
    } finally {
      setLoading(false);
    }
  }

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
