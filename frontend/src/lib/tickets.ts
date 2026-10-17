import { apiFetch } from './api';
import type { Ticket } from '@/types/api';

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
