/**
 * index: pintu masuk aplikasi di "/".
 *
 * Rute        : / (menerima ?next=<path>, yang diteruskan apa adanya).
 * Akses       : publik. Yang sudah login diarahkan ke `next` yang aman atau /dashboard (AuthGate mode guest); tamu
 *               diarahkan ke /login dengan `next` yang sama.
 * Dipakai oleh: URL akar aplikasi, tautan atau bookmark lama ke halaman login di "/".
 * Bergantung  : AuthGate, lib/safe-next, Redirect dari expo-router.
 */
import { Redirect, useLocalSearchParams } from 'expo-router';

import { AuthGate } from '@/components/auth/AuthGate';
import { sanitizeNext } from '@/lib/safe-next';

/**
 * Tidak punya UI sendiri: memilih antara dashboard (sudah login) dan halaman login (tamu).
 */
export default function IndexRoute() {
  // `next` disanitasi di sini juga supaya hanya path internal yang ikut diteruskan ke /login.
  const { next } = useLocalSearchParams<{ next?: string }>();
  const safeNext = sanitizeNext(next, '');
  return (
    <AuthGate mode="guest">
      <Redirect href={safeNext ? `/login?next=${encodeURIComponent(safeNext)}` : '/login'} />
    </AuthGate>
  );
}
