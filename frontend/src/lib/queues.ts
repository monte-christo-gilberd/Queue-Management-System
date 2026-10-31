import { apiFetch } from './api';
import type { Queue } from '@/types/api';

/** Field yang dikirim ke backend untuk buat dan ubah antrian (kontrak: CreateQueueRequest / UpdateQueueRequest). */
export type QueueInput = {
  queue_name: string;
  queue_description: string;
  /** Opsional: 1-3 huruf/angka. Kosong = tidak diubah (ubah) / "A" (buat). */
  ticket_prefix?: string;
};

type ListResponse = { action: 'list_queues'; queues: Queue[] };
type DetailResponse = { action: 'get_queue'; queue: Queue };
type CreateResponse = { action: 'create_queue'; queue: Queue };

/** GET /api/v1/queues — daftar antrian milik owner. Backend hanya mengirim yang aktif bila langganan tidak aktif. */
export async function listQueues(): Promise<Queue[]> {
  const data = await apiFetch<ListResponse>('/api/v1/queues');
  return data.queues;
}

/** GET /api/v1/queues/:id — detail antrian. */
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
