import { router } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import RegisterScreen from '@/components/RegisterScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { useAuthStore } from '@/store/authStore';

export default function RegisterRoute() {
  const register = useAuthStore((s) => s.register);
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
    if (password !== confirmPassword) {
      setErrorMessage('Password dan konfirmasi password harus sama');
      return;
    }
    setLoading(true);
    setErrorMessage(undefined);
    try {
      await register(email.trim(), username.trim(), password);
      router.replace('/dashboard');
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
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
