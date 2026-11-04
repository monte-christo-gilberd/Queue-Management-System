import { useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import ResetPasswordScreen from '@/components/ResetPasswordScreen';
import { describeResetError, isInvalidResetToken, resetPassword } from '@/lib/password-reset';
import { useAuthStore } from '@/store/authStore';
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
  const [loading, setLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();
  const [mode, setMode] = useState<'form' | 'invalid' | 'done'>(rawToken ? 'form' : 'invalid');
  const confirmPasswordError =
    confirmPassword && confirmPassword !== password ? 'Password tidak sama' : undefined;

  async function handleSubmit() {
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
      confirmPasswordError={confirmPasswordError}
      onSubmit={() => void handleSubmit()}
      loading={loading}
      errorMessage={errorMessage}
    />
  );
}
