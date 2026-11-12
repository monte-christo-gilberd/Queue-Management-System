/**
 * verify-email: verifikasi alamat email dari tautan di email (/verify-email?token=...).
 *
 * Rute        : /verify-email (query token wajib).
 * Akses       : publik (tanpa AuthGate); pengguna mungkin belum masuk di perangkat ini.
 * Dipakai oleh: tautan di email verifikasi; banner verifikasi di profil mengirim ulang tautan lewat lib/email-verification.
 * Bergantung  : VerifyEmailScreen, verifyEmail/isInvalidVerifyToken/describeVerifyError (lib/email-verification), useAuthStore (loggedIn).
 * Catatan     : Token sekali pakai, jadi pemeriksaan hanya dijalankan sekali per token.
 */
import { useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';

import VerifyEmailScreen, { type VerifyEmailScreenProps } from '@/components/VerifyEmailScreen';
import { announceEmailVerified, describeVerifyError, isInvalidVerifyToken, verifyEmail } from '@/lib/email-verification';
import { useAuthStore } from '@/store/authStore';

/**
 * Halaman publik (tanpa AuthGate): tautan dibuka dari email, sering di perangkat atau browser lain yang belum
 * masuk. Tautan datang dari email: /verify-email?token=...
 *
 * Token sekali pakai, jadi pemeriksaan hanya dijalankan sekali per token (penjaga `started`), termasuk saat
 * komponen dipasang dua kali oleh mode strict React.
 */
export default function VerifyEmailRoute() {
  const { token } = useLocalSearchParams<{ token?: string | string[] }>();
  const rawToken = (Array.isArray(token) ? token[0] : token) ?? '';
  const loggedIn = useAuthStore((s) => s.token !== null && s.token !== '');
  const started = useRef<string | null>(null);
  const [mode, setMode] = useState<VerifyEmailScreenProps['mode']>(rawToken ? 'loading' : 'invalid');
  const [errorMessage, setErrorMessage] = useState<string>();

  const run = useCallback(async () => {
    setMode('loading');
    setErrorMessage(undefined);
    try {
      await verifyEmail(rawToken);
      setMode('done');
      // Profil di tab lain (bila terbuka) ikut berubah tanpa dimuat ulang.
      announceEmailVerified();
    } catch (e) {
      if (isInvalidVerifyToken(e)) {
        setMode('invalid');
      } else {
        setErrorMessage(describeVerifyError(e));
        setMode('error');
      }
    }
  }, [rawToken]);

  useEffect(() => {
    if (!rawToken || started.current === rawToken) return;
    started.current = rawToken;
    void run();
  }, [rawToken, run]);

  return <VerifyEmailScreen mode={mode} errorMessage={errorMessage} onRetry={() => void run()} loggedIn={loggedIn} />;
}
