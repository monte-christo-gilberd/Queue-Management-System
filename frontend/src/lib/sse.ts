/**
 * Parser SSE dan validasi event antrian. Fungsi murni (tanpa React, tanpa fetch) supaya bisa diuji di Node.
 *
 * Backend mengirim frame `data: {json}\n\n`, komentar `: connected` / `: ping`, dan tidak memakai field `event:`.
 * Tipe event ada di dalam JSON (`type`).
 */

export type SseMessage = { event?: string; data: string };

/** Event dari `GET /api/v1/queues/:id/event`. `payload` bentuknya bergantung pada `type`. */
export type QueueEventType = 'ticket_joined' | 'ticket_next' | 'queue_updated' | 'queue_closed';

export type QueueEvent = {
  type: QueueEventType;
  queue_id: number;
  payload?: unknown;
  ts?: string;
};

const EVENT_TYPES: readonly QueueEventType[] = ['ticket_joined', 'ticket_next', 'queue_updated', 'queue_closed'];

/**
 * Membangun parser incremental. `push` menerima potongan teks apa pun (potongan bisa terpotong di tengah baris).
 * Mendukung akhir baris `\n` dan `\r\n`. Komentar (diawali `:`) diabaikan.
 */
export function createSseParser(onMessage: (message: SseMessage) => void) {
  let buffer = '';
  let dataLines: string[] = [];
  let eventName: string | undefined;

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
    if (value.startsWith(' ')) value = value.slice(1);

    if (field === 'data') dataLines.push(value);
    else if (field === 'event') eventName = value;
    // id dan retry tidak dipakai
  }

  return {
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

/** Mengubah isi `data` menjadi QueueEvent. Mengembalikan null bila JSON rusak atau bentuknya tidak dikenal. */
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

export const BACKOFF_START_MS = 1000;
export const BACKOFF_MAX_MS = 30000;

/** Jeda berikutnya untuk reconnect: dua kali lipat, dibatasi `BACKOFF_MAX_MS`. */
export function nextBackoff(currentMs: number): number {
  return Math.min(Math.max(currentMs, BACKOFF_START_MS) * 2, BACKOFF_MAX_MS);
}
