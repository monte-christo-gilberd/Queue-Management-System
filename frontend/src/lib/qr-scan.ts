const MAX_TOKEN_LENGTH = 2048;

/**
 * Token join dari isi QR hasil scan. QR antrian berisi `<origin>/join?t=<token>`.
 * Mengembalikan null untuk QR lain (bukan URL, path bukan `/join`, atau `t` kosong).
 */
export function parseJoinToken(raw: string): string | null {
  let url: URL;
  try {
    url = new URL(raw.trim());
  } catch {
    return null;
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
  if (url.pathname.replace(/\/+$/, '') !== '/join') return null;
  const token = url.searchParams.get('t');
  if (!token || token.length > MAX_TOKEN_LENGTH) return null;
  return token;
}
