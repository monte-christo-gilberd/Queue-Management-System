import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import LoginScreen from '@/components/LoginScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';

export default function LoginRoute() {
  const login = useAuthStore((s) => s.login);
  // `next` dari AuthGate: halaman yang dituju sebelum diarahkan ke login.
  const { next } = useLocalSearchParams<{ next?: string }>();
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  async function handleSubmit() {
    if (!identifier.trim() || !password) {
      setErrorMessage('Email/username dan password wajib diisi');
      return;
    }
    setLoading(true);
    setErrorMessage(undefined);
    try {
      await login(identifier.trim(), password);
      router.replace(sanitizeNext(next) as '/dashboard');
    } catch (e) {
      setErrorMessage(describeAuthError(e));
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthGate mode="guest">
      <LoginScreen
        identifier={identifier}
        password={password}
        onChangeIdentifier={setIdentifier}
        onChangePassword={setPassword}
        onSubmit={handleSubmit}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
