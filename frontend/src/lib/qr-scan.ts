/**
 * qr-scan.ts: mengambil kode join dari isi QR hasil pemindaian, atau dari kode/tautan yang diketik.
 *
 * Dipakai oleh : components/ui/JoinQueueModals.tsx.
 * Alur         : QR berisi `<origin>/join?t=<kode>`, atau pengunjung mengetik kode 6 karakter ->
 *                parseJoinToken -> kode -> joinByToken (lib/qr.ts).
 */

/** MAX_TOKEN_LENGTH: batas panjang token join (karakter), supaya isi QR yang sangat panjang langsung ditolak. */
const MAX_TOKEN_LENGTH = 2048;

/**
 * Kode join 6 karakter, sama dengan alfabet backend (tanpa 0, O, 1, I, L). Huruf kecil diterima lalu dikapitalkan.
 */
const JOIN_CODE_PATTERN = /^[2-9A-HJKMNP-Z]{6}$/;

/**
 * Kode join dari isi QR hasil scan atau dari isian pengunjung. Menerima:
 * - tautan `<origin>/join?t=<kode>` (isi QR antrian),
 * - kode 6 karakter yang diketik manual (huruf kecil atau besar).
 * Mengembalikan null untuk isian lain.
 * Contoh: `https://x.test/join?t=ab23cd` menghasilkan `AB23CD`; `ab23cd` menghasilkan `AB23CD`;
 * `https://x.test/lain?t=ab23cd` menghasilkan null.
 */
export function parseJoinToken(raw: string): string | null {
  // Buang spasi di tepi; isian dari kamera atau papan tempel sering membawanya.
  const trimmed = raw.trim();
  // Kode 6 karakter yang diketik langsung (tanpa tautan): dikapitalkan lalu dicocokkan dengan alfabet kode.
  const typed = trimmed.toUpperCase();
  if (JOIN_CODE_PATTERN.test(typed)) return typed;

  // Bukan kode: harus berupa URL yang valid.
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return null;
  }
  // Hanya URL web; QR berskema lain (mis. `mailto:`) ditolak.
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return null;
  // Garis miring di akhir path (`/join/`) tetap diterima.
  if (url.pathname.replace(/\/+$/, '') !== '/join') return null;
  // Ambil kode dari parameter `t`; kosong atau terlalu panjang ditolak.
  const code = url.searchParams.get('t');
  if (!code || code.length > MAX_TOKEN_LENGTH) return null;
  return code;
}
