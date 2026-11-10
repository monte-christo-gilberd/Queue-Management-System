/**
 * history.ts: API riwayat antrian (owner) dan riwayat tiket (customer), termasuk ekspor CSV/XLSX.
 *
 * Dipakai oleh : hooks/use-history.ts, components/History.tsx, components/MyHistoryList.tsx, components/HistoryDetail.tsx.
 * Bergantung   : lib/api.ts (apiFetch, apiDownload), lib/download.ts (saveBlobAsFile).
 * Alur         : downloadHistoryExport -> apiDownload (blob) -> saveBlobAsFile; nama berkas dari exportFileName.
 * Catatan      : paging memakai cursor dari next_cursor; null berarti tidak ada halaman berikutnya.
 */
import { apiDownload, apiFetch } from './api';
import { saveBlobAsFile } from './download';
import type { HistoryQueue, HistoryTicket, MyHistoryTicket } from '@/types/api';

/** HistoryQueuesPage: satu halaman daftar antrian riwayat milik owner. */
export type HistoryQueuesPage = {
  /** Antrian pada halaman ini, terbaru dulu. */
  queues: HistoryQueue[];
  /** Kursor halaman berikutnya; null bila sudah halaman terakhir. */
  next_cursor: string | null;
};
/** HistoryQueueDetail: ringkasan satu antrian riwayat beserta satu halaman nomor tiketnya. */
export type HistoryQueueDetail = {
  /** Ringkasan antrian yang sudah ditutup. */
  queue: HistoryQueue;
  /** Nomor tiket pada halaman ini. */
  tickets: HistoryTicket[];
  /** Kursor halaman tiket berikutnya; null bila sudah halaman terakhir. */
  next_cursor: string | null;
};

/** query membangun query string dari cursor dan limit; kosong bila keduanya tidak diisi. */
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

/** ExportFormat: format berkas ekspor riwayat (`csv` atau `xlsx`). */
export type ExportFormat = 'csv' | 'xlsx';

/** Path ekspor riwayat satu antrian (fungsi murni, untuk tes). */
export function exportHistoryPath(queueId: number, format: ExportFormat): string {
  return `/api/v1/history/queues/${queueId}/export?format=${format}`;
}

/**
 * Nama berkas unduhan. Aturannya sama dengan export.FileName di backend: huruf kecil ASCII, angka, tanda hubung.
 * Contoh: ("Loket Utama!", 54, "csv") menghasilkan "riwayat-loket-utama-54.csv".
 */
export function exportFileName(queueName: string, queueId: number, format: ExportFormat): string {
  let slug = queueName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
  if (slug.length > 40) slug = slug.slice(0, 40).replace(/^-+|-+$/g, '');
  // Nama yang hanya berisi simbol menghasilkan slug kosong; pakai nama cadangan.
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

/** MyHistoryPage: satu halaman riwayat tiket milik customer yang sedang login. */
export type MyHistoryPage = {
  /** Tiket pada halaman ini, satu baris per tiket, terbaru dulu. */
  tickets: MyHistoryTicket[];
  /** Kursor halaman berikutnya; null bila sudah halaman terakhir. */
  next_cursor: string | null;
};

/** Path riwayat tiket customer (fungsi murni, untuk tes). */
export function myHistoryPath(cursor?: string | null, limit?: number): string {
  return `/api/v1/users/me/history${query(cursor, limit)}`;
}

/** GET /api/v1/users/me/history — riwayat tiket saya, satu baris per tiket, terbaru dulu, per halaman. */
export async function listMyHistory(cursor?: string | null, limit?: number): Promise<MyHistoryPage> {
  return apiFetch<MyHistoryPage>(myHistoryPath(cursor, limit));
}
