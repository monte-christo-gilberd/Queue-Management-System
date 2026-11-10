/**
 * index: halaman login di "/", untuk tamu yang belum masuk.
 *
 * Rute        : / (menerima ?next=<path> dari AuthGate atau tautan register).
 * Akses       : guest-only (AuthGate mode guest); yang sudah punya token diarahkan ke tujuan next atau /dashboard.
 * Dipakai oleh: AuthGate di rute yang butuh login (mengirim ?next=), login.tsx (redirect dari /login), register.tsx (tautan sudah punya akun).
 * Bergantung  : useAuthStore (login), LoginScreen, lib/safe-next, lib/auth-errors.
 * Catatan     : Tujuan setelah login: next yang sudah disanitasi, atau DEFAULT_AFTER_LOGIN (/dashboard) bila kosong.
 */
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';

import { AuthGate } from '@/components/auth/AuthGate';
import LoginScreen from '@/components/LoginScreen';
import { describeAuthError } from '@/lib/auth-errors';
import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';
import { useAuthStore } from '@/store/authStore';

/**
 * Halaman login. Mengirim identifier (email atau username) dan password ke auth store, lalu pindah ke tujuan setelah berhasil.
 */
export default function LoginRoute() {
  const login = useAuthStore((s) => s.login);
  // `next` dari AuthGate: halaman yang dituju sebelum diarahkan ke login.
  const { next } = useLocalSearchParams<{ next?: string }>();
  const safeNext = sanitizeNext(next, '');
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
      <LoginScreen
        identifier={identifier}
        password={password}
        onChangeIdentifier={setIdentifier}
        onChangePassword={setPassword}
        onSubmit={handleSubmit}
        registerHref={safeNext ? `/register?next=${encodeURIComponent(safeNext)}` : '/register'}
        loading={loading}
        errorMessage={errorMessage}
      />
    </AuthGate>
  );
}
