/**
 * qr-scan.ts: mengambil token join dari isi QR hasil pemindaian.
 *
 * Dipakai oleh : components/ui/JoinQueueModals.tsx.
 * Alur         : QR berisi `<origin>/join?t=<token>` -> parseJoinToken -> token -> previewJoin (lib/qr.ts).
 */

/** MAX_TOKEN_LENGTH: batas panjang token join (karakter), supaya isi QR yang sangat panjang langsung ditolak. */
const MAX_TOKEN_LENGTH = 2048;

/**
 * Token join dari isi QR hasil scan. QR antrian berisi `<origin>/join?t=<token>`.
 * Mengembalikan null untuk QR lain (bukan URL, path bukan `/join`, atau `t` kosong).
 * Contoh: `https://x.test/join?t=abc` menghasilkan `abc`; `https://x.test/lain?t=abc` menghasilkan null.
 */
export function parseJoinToken(raw: string): string | null {
  let url: URL;
  try {
    url = new URL(raw.trim());
  } catch {
    return null;
  }
  // Hanya URL web; QR berskema lain (mis. `mailto:`) ditolak.
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
  // Garis miring di akhir path (`/join/`) tetap diterima.
  if (url.pathname.replace(/\/+$/, '') !== '/join') return null;
  const token = url.searchParams.get('t');
  if (!token || token.length > MAX_TOKEN_LENGTH) return null;
  return token;
}
