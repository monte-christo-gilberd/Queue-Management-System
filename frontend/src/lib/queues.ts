import { apiFetch } from './api';
import type { Queue, QueueAuthz } from '@/types/api';

/** Field yang dikirim ke backend untuk buat dan ubah antrian (kontrak: CreateQueueRequest / UpdateQueueRequest). */
export type QueueInput = {
  queue_name: string;
  queue_description: string;
};

type ListResponse = { action: 'list_queues'; queues: Queue[] };
type DetailResponse = { action: 'get_queue'; queue: Queue; queue_authz: QueueAuthz };
type CreateResponse = { action: 'create_queue'; queue: Queue; queue_authz: QueueAuthz };

/** GET /api/v1/queues — daftar antrian milik owner. Backend hanya mengirim yang aktif bila langganan tidak aktif. */
export async function listQueues(): Promise<Queue[]> {
  const data = await apiFetch<ListResponse>('/api/v1/queues');
  return data.queues;
}

/** GET /api/v1/queues/:id — detail antrian beserta izin (queue_authz). */
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

/** PUT /api/v1/queues/:id/authz — ubah izin (authz) pada antrian ini. Backend menimpa semua flag sekaligus. */
export async function updateAuthz(queueId: number, input: QueueAuthz): Promise<void> {
  await apiFetch(`/api/v1/queues/${queueId}/authz`, { method: 'PUT', body: { ...input, queue_id: queueId } });
}

/** POST /api/v1/queues/:id/close — tutup antrian. Tidak bisa dibuka lagi. */
export async function closeQueue(queueId: number): Promise<void> {
  await apiFetch(`/api/v1/queues/${queueId}/close`, { method: 'POST' });
}
