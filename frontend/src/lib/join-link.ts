/**
 * Tautan join: `<origin>/join/<queueId>`. Fungsi murni kecuali `buildJoinLink` yang membaca origin.
 */

/** Alamat web aplikasi. Di web: origin halaman. Di native: EXPO_PUBLIC_WEB_URL (kosong = tidak tersedia). */
export function buildJoinLink(queueId: number, webOrigin: string | null): string | null {
  if (!webOrigin || !Number.isInteger(queueId) || queueId <= 0) return null;
  return `${webOrigin.replace(/\/+$/, '')}/join/${queueId}`;
}

/**
 * Mengambil id antrian dari tautan penuh (`https://…/join/12`), path (`/join/12`), atau angka (`12`).
 * Mengembalikan null bila tidak dikenali. Hanya angka positif yang diterima.
 */
export function parseQueueIdFromLink(input: string): number | null {
  const text = input.trim();
  if (text === '') return null;

  const fromPath = text.match(/\/join\/(\d+)\/?(?:[?#].*)?$/);
  if (fromPath) return toId(fromPath[1]);

  if (/^\d+$/.test(text)) return toId(text);
  return null;
}

function toId(digits: string): number | null {
  const id = Number(digits);
  return Number.isSafeInteger(id) && id > 0 ? id : null;
}
