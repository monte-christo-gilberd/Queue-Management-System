/**
 * auth-gate.ts: keputusan redirect untuk AuthGate (wait, render, atau redirect).
 *
 * Dipakai oleh : components/auth/AuthGate.tsx.
 * Bergantung   : lib/safe-next.ts (sanitizeNext).
 * Alur         : AuthGate memanggil decideAuthGate dengan status hidrasi dan token. Mode `required` yang belum
 *                login diarahkan ke `/?next=<path>`; mode `guest` yang sudah login diarahkan ke `next` yang aman.
 * Catatan      : lihat AUTH_GATE_GUIDE.md.
 */

/**
 * Keputusan redirect untuk AuthGate. Fungsi murni (tanpa React / router) supaya bisa diuji di Node.
 *
 * - `wait`: storage belum selesai dibaca, jangan memutuskan apa pun (cegah redirect sekilas).
 * - `render`: boleh menampilkan children.
 * - `redirect`: arahkan ke `to`.
 */
import { sanitizeNext } from './safe-next';

/** AuthGateMode: `required` untuk halaman yang butuh login; `guest` untuk halaman yang hanya untuk yang belum masuk. */
export type AuthGateMode = 'required' | 'guest';

/** AuthGateDecision: hasil keputusan AuthGate; lihat keterangan di atas. */
export type AuthGateDecision =
  | { kind: 'wait' }
  | { kind: 'render' }
  /** Path tujuan redirect (sudah termasuk query bila ada). */
  | { kind: 'redirect'; to: string };

/** LOGIN_PATH: route halaman login. */
export const LOGIN_PATH = '/';
/** HOME_PATH: halaman utama owner, tujuan default setelah login. */
export const HOME_PATH = '/dashboard';

/**
 * decideAuthGate memutuskan apakah AuthGate menunggu, merender, atau mengarahkan.
 * Contoh: mode `required`, hydrated true, token null, path `/dashboard` menghasilkan redirect ke `/?next=%2Fdashboard`.
 * Mode `guest` dengan token terisi dan next `/dashboard` menghasilkan redirect ke `/dashboard`.
 */
export function decideAuthGate(input: {
  /** Mode halaman: `required` (wajib login) atau `guest` (login/register). */
  mode: AuthGateMode;
  /** true setelah storage auth selesai dibaca; sebelum itu keputusan selalu `wait`. */
  hydrated: boolean;
  /** Token sesi; null atau string kosong berarti belum login. */
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

  // Token string kosong dianggap belum login.
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
