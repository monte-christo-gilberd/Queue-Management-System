import { apiFetch } from './api';
import type { PublicQueue, QueueStatus, Ticket } from '@/types/api';

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
    previous_number: data.previous_number,
    next_number: data.next_number,
  };
}

type MyTicketsResponse = { action: 'list_my_tickets'; tickets: MyTicket[] };

/** Tiket saya di semua antrian, beserta nama dan status antriannya. */
export type MyTicket = Ticket & {
  queue_name: string;
  queue_status: 'active' | 'closed';
  /** ticket_number yang sedang dilayani di antrian ini; null bila belum ada. */
  current_number: number | null;
};

/** Tiket yang masih relevan di beranda: menunggu di antrian aktif, atau nomornya sedang dilayani. */
export function isOnHome(t: MyTicket): boolean {
  if (t.queue_status !== 'active') return false;
  return t.status === 'waiting' || (t.status === 'completed' && t.current_number === t.ticket_number);
}

/** GET /api/v1/users/me/tickets — tiket saya (login diperlukan). */
export async function listMyTickets(): Promise<MyTicket[]> {
  const data = await apiFetch<MyTicketsResponse>('/api/v1/users/me/tickets');
  return data.tickets;
}

/** GET /api/v1/queues/:id/info — nama, deskripsi, dan status antrian (login diperlukan, tanpa owner/izin). */
export async function getQueueInfo(queueId: number): Promise<PublicQueue> {
  const data = await apiFetch<{ action: 'queue_info'; queue: PublicQueue }>(`/api/v1/queues/${queueId}/info`);
  return data.queue;
}
