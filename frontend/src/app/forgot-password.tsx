import { router } from 'expo-router';
import { useState } from 'react';

import ForgotPasswordScreen from '@/components/ForgotPasswordScreen';

export default function ForgotPasswordRoute() {
  const [email, setEmail] = useState('');

  return (
    <ForgotPasswordScreen
      email={email}
      onChangeEmail={setEmail}
      // TODO(developer): ganti dengan permintaan reset password (kirim email). Sementara hanya kembali ke login.
      onSubmit={() => router.replace('/')}
      loading={false}
    />
  );
}
