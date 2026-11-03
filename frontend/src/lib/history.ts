import { apiFetch } from './api';
import type { HistoryQueue, HistoryTicket } from '@/types/api';

export type HistoryQueuesPage = { queues: HistoryQueue[]; next_cursor: string | null };
export type HistoryQueueDetail = { queue: HistoryQueue; tickets: HistoryTicket[]; next_cursor: string | null };

function query(cursor?: string | null, limit?: number): string {
  const params: string[] = [];
  if (limit) params.push(`limit=${limit}`);
  if (cursor) params.push(`cursor=${encodeURIComponent(cursor)}`);
  return params.length ? `?${params.join('&')}` : '';
}

/** Path daftar riwayat (fungsi murni, untuk tes). */
export function historyQueuesPath(cursor?: string | null, limit?: number): string {
  return `/api/v1/history/queues${query(cursor, limit)}`;
}

/** Path detail riwayat satu antrian (fungsi murni, untuk tes). */
export function historyQueuePath(queueId: number, cursor?: string | null, limit?: number): string {
  return `/api/v1/history/queues/${queueId}${query(cursor, limit)}`;
}

/** GET /api/v1/history/queues — antrian milik owner yang sudah ditutup, terbaru dulu, per halaman. */
export async function listHistoryQueues(cursor?: string | null, limit?: number): Promise<HistoryQueuesPage> {
  return apiFetch<HistoryQueuesPage>(historyQueuesPath(cursor, limit));
}

/** GET /api/v1/history/queues/:id — ringkasan dan satu halaman nomor antrian yang sudah ditutup (owner). */
export async function getHistoryQueue(
  queueId: number,
  cursor?: string | null,
  limit?: number,
): Promise<HistoryQueueDetail> {
  return apiFetch<HistoryQueueDetail>(historyQueuePath(queueId, cursor, limit));
}
