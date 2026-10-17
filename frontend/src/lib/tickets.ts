import { apiFetch } from './api';
import type { QueueStatus, Ticket } from '@/types/api';

type ListTicketsResponse = { action: 'list_tickets'; queue_id: number; tickets: Ticket[] };

/** GET /api/v1/queues/:id/tickets — semua tiket antrian (owner). Backend membatasi 500 baris, urut sort_key. */
export async function listTickets(queueId: number): Promise<Ticket[]> {
  const data = await apiFetch<ListTicketsResponse>(`/api/v1/queues/${queueId}/tickets`);
  return data.tickets;
}

/**
 * POST /api/v1/queues/:id/next — panggil tiket waiting pertama (owner).
 * Respons join/next tidak dibungkus `action`, langsung objek tiket di dalam `data`.
 * 409 QUEUE_EMPTY bila tidak ada yang menunggu.
 */
export async function nextTicket(queueId: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/next`, { method: 'POST' });
}

/**
 * POST /api/v1/queues/:id/join — customer bergabung ke antrian. Respons langsung objek tiket.
 * 409 ALREADY_JOINED / QUEUE_CLOSED, 404 bila antrian tidak ada, 503 RETRY_EXHAUSTED bila sibuk.
 */
export async function joinQueue(queueId: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/join`, { method: 'POST' });
}

/**
 * GET /api/v1/queues/:id/status — status untuk pemegang tiket. Untuk yang belum punya tiket backend
 * membalas 404 (sama dengan antrian tidak ada); pemanggil memperlakukannya sebagai "belum bergabung".
 */
export async function getQueueStatus(queueId: number): Promise<QueueStatus> {
  const data = await apiFetch<{ action: 'queue_status' } & QueueStatus>(`/api/v1/queues/${queueId}/status`);
  return {
    queue: data.queue,
    my_ticket: data.my_ticket,
    current_number: data.current_number,
    waiting_ahead: data.waiting_ahead,
  };
}
