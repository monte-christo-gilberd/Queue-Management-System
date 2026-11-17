/**
 * queues.ts: panggilan API antrian untuk owner (daftar, detail, buat, ubah, tutup).
 *
 * Dipakai oleh : hooks/use-queues.ts, app/(user)/dashboard.tsx, app/(user)/queues/[queueId]/index.tsx
 * Bergantung   : lib/api.ts (apiFetch), types/api.ts (Queue)
 * Catatan      : Semua fungsi butuh login owner. Galat dibaca dari ApiError (lib/api.ts).
 */
import { apiFetch } from './api';
import type { Queue } from '@/types/api';

/** QueueInput: field yang dikirim ke backend untuk buat dan ubah antrian (kontrak: CreateQueueRequest / UpdateQueueRequest). */
export type QueueInput = {
  /** Nama antrian yang tampil ke customer. Wajib diisi dan tidak boleh berisi tag HTML. */
  queue_name: string;
  /** Deskripsi singkat antrian. Boleh kosong. */
  queue_description: string;
  /** Opsional: 1-3 huruf/angka. Kosong = tidak diubah (ubah) / "A" (buat). */
  ticket_prefix?: string;
};

/** Bentuk `data` dari GET /api/v1/queues. `action` penanda jenis respons dari backend, tidak dibaca di frontend. */
type ListResponse = {
  /** Penanda respons dari backend. */
  action: 'list_queues';
  /** Semua antrian milik owner. */
  queues: Queue[];
};

/** Bentuk `data` dari GET /api/v1/queues/:id. */
type DetailResponse = {
  /** Penanda respons dari backend. */
  action: 'get_queue';
  /** Antrian yang diminta. */
  queue: Queue;
};

/** Bentuk `data` dari POST /api/v1/queues. */
type CreateResponse = {
  /** Penanda respons dari backend. */
  action: 'create_queue';
  /** Antrian yang baru dibuat. */
  queue: Queue;
};

/** GET /api/v1/queues — daftar antrian milik owner. Backend hanya mengirim yang aktif bila langganan tidak aktif. */
export async function listQueues(): Promise<Queue[]> {
  const data = await apiFetch<ListResponse>('/api/v1/queues');
  return data.queues;
}

/** GET /api/v1/queues/:id — detail antrian. Mengembalikan respons utuh (dengan `action`), bukan langsung Queue. */
export async function getQueue(queueId: number): Promise<DetailResponse> {
  return apiFetch<DetailResponse>(`/api/v1/queues/${queueId}`);
}

/** POST /api/v1/queues — buat antrian. Melempar ApiError 402 bila langganan tidak aktif. */
export async function createQueue(input: QueueInput): Promise<Queue> {
  const data = await apiFetch<CreateResponse>('/api/v1/queues', { method: 'POST', body: input });
  return data.queue;
}

/** PUT /api/v1/queues/:id — ubah nama dan deskripsi. Field kosong = tetap seperti semula (backend). */
export async function updateQueue(queueId: number, input: QueueInput): Promise<void> {
  await apiFetch(`/api/v1/queues/${queueId}`, { method: 'PUT', body: input });
}

/** POST /api/v1/queues/:id/close — tutup antrian. Tidak bisa dibuka lagi. */
export async function closeQueue(queueId: number): Promise<void> {
  await apiFetch(`/api/v1/queues/${queueId}/close`, { method: 'POST' });
}
