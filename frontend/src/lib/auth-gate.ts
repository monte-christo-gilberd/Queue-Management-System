/**
 * Keputusan redirect untuk AuthGate. Fungsi murni (tanpa React / router) supaya bisa diuji di Node.
 *
 * - `wait`: storage belum selesai dibaca, jangan memutuskan apa pun (cegah redirect sekilas).
 * - `render`: boleh menampilkan children.
 * - `redirect`: arahkan ke `to`.
 */
import { sanitizeNext } from './safe-next';

export type AuthGateMode = 'required' | 'guest';

export type AuthGateDecision =
  | { kind: 'wait' }
  | { kind: 'render' }
  | { kind: 'redirect'; to: string };

/** Halaman login (route `/`) dan halaman utama owner. */
export const LOGIN_PATH = '/';
export const HOME_PATH = '/dashboard';

export function decideAuthGate(input: {
  mode: AuthGateMode;
  hydrated: boolean;
  token: string | null;
  /** Path saat ini (mis. `/dashboard`). Dipakai untuk `?next=` saat dilempar ke login. */
  path?: string;
  /**
   * Tujuan eksplisit setelah login, mis. `/join?t=<token>`. `usePathname()` tidak membawa
   * query string, jadi halaman yang punya query penting (token join) mengirimkannya sendiri.
   */
  nextPath?: string;
  /** Parameter `next` dari URL halaman guest (login/register). Dipakai bila sudah login. */
  next?: unknown;
}): AuthGateDecision {
  if (!input.hydrated) return { kind: 'wait' };

  const loggedIn = input.token !== null && input.token !== '';

  if (input.mode === 'required') {
    if (loggedIn) return { kind: 'render' };
    // nextPath eksplisit menang agar query (mis. `?t=<token>`) tidak hilang.
    const path = input.nextPath || input.path;
    // Simpan halaman asal supaya bisa kembali setelah login. Tidak untuk `/` (sudah di login).
    if (path && path !== LOGIN_PATH) {
      return { kind: 'redirect', to: `${LOGIN_PATH}?next=${encodeURIComponent(path)}` };
    }
    return { kind: 'redirect', to: LOGIN_PATH };
  }
  // guest: halaman login/register hanya untuk yang belum masuk. Yang sudah masuk diteruskan ke `next` (bila aman).
  return loggedIn ? { kind: 'redirect', to: sanitizeNext(input.next, HOME_PATH) } : { kind: 'render' };
}
