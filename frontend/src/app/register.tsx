/**
 * register: form daftar akun baru untuk tamu; setelah berhasil diarahkan ke next yang aman atau ke dashboard.
 *
 * Rute        : /register (query next opsional).
 * Akses       : guest-only (AuthGate mode guest).
 * Dipakai oleh: tautan Daftar di halaman login (/), yang membawa next bila ada.
 * Bergantung  : RegisterScreen, useAuthStore (register), validatePassword (utils/password), sanitizeNext.
 * Catatan     : Validasi password dan konfirmasinya dilakukan di route, bukan di komponen tampilan.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import RegisterScreen from '@/components/RegisterScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';
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
      // Cast hanya agar tipe rute yang ketat menerima string dinamis dari ?next atau nilai default.
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
        next={safeNext}
        loginHref={safeNext ? `/?next=${encodeURIComponent(safeNext)}` : '/'}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
