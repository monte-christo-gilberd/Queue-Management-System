import { useState } from 'react';

import ForgotPasswordScreen from '@/components/ForgotPasswordScreen';
import { describeForgotError, requestPasswordReset } from '@/lib/password-reset';

const SENT_MESSAGE =
  'Jika email itu terdaftar, tautan untuk membuat password baru sudah kami kirim. Periksa kotak masuk (dan folder spam) Anda.';

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
