import { apiDownload, apiFetch } from './api';
import { saveBlobAsFile } from './download';
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

export type ExportFormat = 'csv' | 'xlsx';

/** Path ekspor riwayat satu antrian (fungsi murni, untuk tes). */
export function exportHistoryPath(queueId: number, format: ExportFormat): string {
  return `/api/v1/history/queues/${queueId}/export?format=${format}`;
}

/** Nama berkas unduhan. Aturannya sama dengan export.FileName di backend: huruf kecil ASCII, angka, tanda hubung. */
export function exportFileName(queueName: string, queueId: number, format: ExportFormat): string {
  let slug = queueName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  if (slug.length > 40) slug = slug.slice(0, 40).replace(/^-+|-+$/g, '');
  if (!slug) slug = 'antrian';
  return `riwayat-${slug}-${queueId}.${format}`;
}

/** Mengunduh ekspor riwayat (CSV atau Excel) antrian yang sudah ditutup. Hanya web; native melempar galat jelas. */
export async function downloadHistoryExport(
  queue: { id: number; queue_name: string },
  format: ExportFormat,
): Promise<void> {
  const blob = await apiDownload(exportHistoryPath(queue.id, format));
  saveBlobAsFile(blob, exportFileName(queue.queue_name, queue.id, format));
}
