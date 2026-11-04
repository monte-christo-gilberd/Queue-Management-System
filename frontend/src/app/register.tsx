import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import RegisterScreen from '@/components/RegisterScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';
import { validatePassword } from '@/utils/password';

export default function RegisterRoute() {
  const register = useAuthStore((s) => s.register);
  // `next` dari tautan join: setelah daftar, kembali ke halaman itu (bukan dashboard).
  const { next } = useLocalSearchParams<{ next?: string }>();
  const safeNext = sanitizeNext(next, '');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  // Pengecekan kesamaan password ada di route (bukan di komponen tampilan).
  const confirmPasswordError =
    confirmPassword && confirmPassword !== password ? 'Password tidak sama' : undefined;

  async function handleSubmit() {
    if (!username.trim() || !email.trim() || !password) {
      setErrorMessage('Username, email, dan password wajib diisi');
      return;
    }
    const weak = validatePassword(password);
    if (weak) {
      setErrorMessage(weak);
      return;
    }
    if (password !== confirmPassword) {
      setErrorMessage('Password dan konfirmasi password harus sama');
      return;
    }
    setLoading(true);
    setErrorMessage(undefined);
    try {
      await register(email.trim(), username.trim(), password);
      router.replace((safeNext || DEFAULT_AFTER_LOGIN) as '/dashboard');
    } catch (e) {
      setErrorMessage(describeAuthError(e));
    } finally {
      setLoading(false);
    }
  }

  return (
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
        confirmPasswordError={confirmPasswordError}
        onSubmit={handleSubmit}
        loginHref={safeNext ? `/?next=${encodeURIComponent(safeNext)}` : '/'}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
