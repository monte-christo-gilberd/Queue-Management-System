/**
 * sse.ts: parser SSE dan validasi event antrian. Fungsi murni (tanpa React, tanpa fetch) supaya bisa diuji di Node.
 *
 * Dipakai oleh : hooks/use-queue-events.ts, hooks/use-display-screen.ts
 * Bergantung   : tidak ada (fungsi murni)
 * Alur         : hook membuat parser lalu memanggil push() untuk setiap potongan stream. use-queue-events meneruskan
 *                frame ke parseQueueEvent. Jeda reconnect dan resinkron memakai nextBackoff, resyncInterval, dan shouldResync.
 * Catatan      : Backend mengirim frame `data: {json}\n\n` dan komentar `: connected` / `: ping`, tanpa field `event:`.
 *                Tipe event ada di dalam JSON (`type`).
 */

/** SseMessage: satu frame SSE yang sudah diurai, sebelum isinya diperiksa sebagai JSON. */
export type SseMessage = {
  /** Nama dari field `event:`. undefined bila frame tidak punya field itu, dan backend ini memang tidak mengirimnya. */
  event?: string;
  /** Gabungan semua baris `data:` dalam frame, dipisah `\n`. Belum di-parse sebagai JSON. */
  data: string;
};

/** Event dari `GET /api/v1/queues/:id/event`. `payload` bentuknya bergantung pada `type`. */
export type QueueEventType =
  | 'ticket_joined'
  | 'ticket_next'
  | 'ticket_skipped'
  | 'ticket_cancelled'
  | 'ticket_removed'
  | 'counter_updated'
  | 'queue_updated'
  | 'queue_closed'
  | 'queue_qr_updated';

/** QueueEvent: event antrian yang sudah divalidasi oleh parseQueueEvent. */
export type QueueEvent = {
  /** Jenis event. Hanya nilai dari QueueEventType yang lolos validasi. */
  type: QueueEventType;
  /** ID antrian tempat event terjadi. Selalu bilangan bulat. */
  queue_id: number;
  /** Data tambahan yang bentuknya bergantung pada `type`. undefined bila backend tidak mengirimnya. */
  payload?: unknown;
  /** Waktu event dari server (string). undefined bila tidak ada atau bukan string. */
  ts?: string;
};

/** Daftar jenis event yang diterima. Harus sama dengan QueueEventType, dipakai untuk validasi di parseQueueEvent. */
const EVENT_TYPES: readonly QueueEventType[] = [
  'ticket_joined',
  'ticket_next',
  'ticket_skipped',
  'ticket_cancelled',
  'ticket_removed',
  'counter_updated',
  'queue_updated',
  'queue_closed',
  'queue_qr_updated',
];

/**
 * createSseParser membangun parser incremental. `push` menerima potongan teks apa pun (potongan bisa terpotong di tengah baris).
 * Mendukung akhir baris `\n` dan `\r\n`. Komentar (diawali `:`) diabaikan.
 */
export function createSseParser(onMessage: (message: SseMessage) => void) {
  let buffer = '';
  let dataLines: string[] = [];
  let eventName: string | undefined;

  // Baris kosong mengakhiri frame. Frame tanpa baris data (mis. hanya komentar) tidak menghasilkan pesan.
  function dispatch() {
    if (dataLines.length > 0) {
      onMessage({ event: eventName, data: dataLines.join('\n') });
    }
    dataLines = [];
    eventName = undefined;
  }

  function processLine(line: string) {
    if (line === '') {
      dispatch();
      return;
    }
    if (line.startsWith(':')) return; // komentar / keep-alive

    const colon = line.indexOf(':');
    const field = colon === -1 ? line : line.slice(0, colon);
    let value = colon === -1 ? '' : line.slice(colon + 1);
    // Satu spasi setelah titik dua bagian dari format SSE, bukan isi nilai.
    if (value.startsWith(' ')) value = value.slice(1);

    if (field === 'data') dataLines.push(value);
    else if (field === 'event') eventName = value;
    // id dan retry tidak dipakai
  }

  return {
    /** Menambah potongan stream. Baris yang belum lengkap disimpan di buffer sampai potongan berikutnya datang. */
    push(chunk: string) {
      buffer += chunk;
      let newline = buffer.indexOf('\n');
      while (newline !== -1) {
        let line = buffer.slice(0, newline);
        if (line.endsWith('\r')) line = line.slice(0, -1);
        buffer = buffer.slice(newline + 1);
        processLine(line);
        newline = buffer.indexOf('\n');
      }
    },
  };
}

/**
 * Mengubah isi `data` menjadi QueueEvent. Mengembalikan null bila JSON rusak atau bentuknya tidak dikenal.
 * Bentuk yang diterima: `{ "type": "...", "queue_id": 1, "payload": ..., "ts": "..." }`.
 */
export function parseQueueEvent(data: string): QueueEvent | null {
  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null) return null;

  const { type, queue_id, payload, ts } = value as Record<string, unknown>;
  if (typeof type !== 'string' || !(EVENT_TYPES as readonly string[]).includes(type)) return null;
  if (typeof queue_id !== 'number' || !Number.isInteger(queue_id)) return null;

  return {
    type: type as QueueEventType,
    queue_id,
    payload,
    ts: typeof ts === 'string' ? ts : undefined,
  };
}

/** Jeda reconnect pertama dalam milidetik. */
export const BACKOFF_START_MS = 1000;

/** Jeda reconnect terbesar dalam milidetik. */
export const BACKOFF_MAX_MS = 30000;

/**
 * nextBackoff: jeda reconnect berikutnya, dua kali lipat dan dibatasi `BACKOFF_MAX_MS`.
 * Contoh: nextBackoff(1000) = 2000, nextBackoff(20000) = 30000.
 */
export function nextBackoff(currentMs: number): number {
  return Math.min(Math.max(currentMs, BACKOFF_START_MS) * 2, BACKOFF_MAX_MS);
}

/** Jeda resinkron normal (ms) saat SSE hidup. */
export const RESYNC_INTERVAL_MS = 45000;

/** SSE tidak hidup (mis. proxy/tunnel menahan stream): ambil ulang lebih sering. */
export const FALLBACK_POLL_MS = 3000;

/** Stream dianggap tidak hidup bila belum ada satu byte pun setelah selama ini. */
export const SSE_LIVE_GRACE_MS = 3000;

/**
 * resyncInterval: interval resinkron dalam ms. Cepat (FALLBACK_POLL_MS) bila SSE tidak hidup dan sudah lewat
 * SSE_LIVE_GRACE_MS sejak `since`, normal bila hidup. Contoh: resyncInterval(false, 0, 5000) = 3000,
 * resyncInterval(true, 0, 5000) = 45000.
 */
export function resyncInterval(live: boolean, since: number, now: number): number {
  return !live && now - since >= SSE_LIVE_GRACE_MS ? FALLBACK_POLL_MS : RESYNC_INTERVAL_MS;
}

/**
 * shouldResync: perlu resinkron? Event SSE bisa terbuang (channel hub penuh) atau tertunda saat tab di latar belakang.
 * Kembali aktif selalu memicu; saat aktif, memicu bila sudah `interval` sejak sinkron terakhir (waktu dalam ms).
 * Contoh: shouldResync(true, false, 0, 50000) = true, shouldResync(false, true, 0, 0) = false.
 */
export function shouldResync(
  active: boolean,
  becameActive: boolean,
  lastSyncAt: number,
  now: number,
  interval: number = RESYNC_INTERVAL_MS,
): boolean {
  if (!active) return false;
  if (becameActive) return true;
  return now - lastSyncAt >= interval;
}
