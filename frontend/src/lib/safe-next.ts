/**
 * safe-next.ts: membersihkan parameter `next` agar tidak bisa dipakai untuk open redirect.
 *
 * Dipakai oleh : lib/auth-gate.ts, app/index.tsx, app/login.tsx, app/register.tsx.
 * Alur         : `?next=` dari URL login/register -> sanitizeNext -> tujuan redirect setelah login.
 * Catatan      : DEFAULT_AFTER_LOGIN bernilai sama dengan HOME_PATH di lib/auth-gate.ts; ubah keduanya bersama.
 */

/**
 * Membersihkan parameter `next` (path kembali setelah login) agar tidak bisa dipakai
 * untuk mengarahkan user ke situs lain (open redirect).
 *
 * Hanya path internal yang diterima: diawali satu `/`, bukan `//`, tanpa backslash, tanpa skema.
 * Selain itu dikembalikan `fallback`.
 */

/** DEFAULT_AFTER_LOGIN: tujuan default setelah login bila `next` tidak valid. */
export const DEFAULT_AFTER_LOGIN = '/dashboard';

/**
 * sanitizeNext mengembalikan `raw` bila berupa path internal yang aman, selain itu `fallback`.
 * Contoh: `/antrian?x=1` dikembalikan apa adanya; `//evil.com`, `https://evil.com`, `/`, dan `/login` menghasilkan fallback.
 */
export function sanitizeNext(raw: unknown, fallback: string = DEFAULT_AFTER_LOGIN): string {
  // fallback '' = "tidak ada next" (dipakai halaman login/register untuk meneruskan parameter)
  if (typeof raw !== 'string') return fallback;
  if (!raw.startsWith('/')) return fallback;
  if (raw.startsWith('//')) return fallback;
  if (raw.includes('\\')) return fallback;
  if (/[\u0000-\u001f]/.test(raw)) return fallback;
  // path yang kembali ke pintu masuk atau halaman login sendiri tidak berguna
  if (raw === '/' || raw.startsWith('/?')) return fallback;
  if (raw === '/login' || raw.startsWith('/login?') || raw.startsWith('/login/')) return fallback;
  return raw;
}
