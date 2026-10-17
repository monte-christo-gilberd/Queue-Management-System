/**
 * Membersihkan parameter `next` (path kembali setelah login) agar tidak bisa dipakai
 * untuk mengarahkan user ke situs lain (open redirect).
 *
 * Hanya path internal yang diterima: diawali satu `/`, bukan `//`, tanpa backslash, tanpa skema.
 * Selain itu dikembalikan `fallback`.
 */
export const DEFAULT_AFTER_LOGIN = '/dashboard';

export function sanitizeNext(raw: unknown, fallback: string = DEFAULT_AFTER_LOGIN): string {
  // fallback '' = "tidak ada next" (dipakai halaman login/register untuk meneruskan parameter)
  if (typeof raw !== 'string') return fallback;
  if (!raw.startsWith('/')) return fallback;
  if (raw.startsWith('//')) return fallback;
  if (raw.includes('\\')) return fallback;
  if (/[\u0000-\u001f]/.test(raw)) return fallback;
  // path yang kembali ke halaman login sendiri tidak berguna
  if (raw === '/' || raw.startsWith('/?')) return fallback;
  return raw;
}
