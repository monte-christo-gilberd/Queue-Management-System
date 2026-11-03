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
export async function nextTicket(queueId: number, counterId = 0): Promise<Ticket> {
  // counter_id 0 = counter default milik queue (backend mengalaminya).
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/next`, { method: 'POST', body: { counter_id: counterId } });
}

/**
 * POST /api/v1/queues/:id/skip — lewati nomor yang sedang dilayani dan panggil berikutnya (owner).
 * Mengembalikan tiket yang baru dipanggil. 409 NOTHING_TO_SKIP bila belum ada yang dilayani,
 * 409 QUEUE_EMPTY bila tidak ada yang menunggu (nomor tetap dilayani).
 */
/** POST /api/v1/queues/:id/recall — owner memanggil ulang nomor yang sedang dilayani (push saja). */
export async function recallTicket(queueId: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/recall`, { method: 'POST' });
}

export async function skipTicket(queueId: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/skip`, { method: 'POST' });
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
    current_counter_name: data.current_counter_name,
    waiting_ahead: data.waiting_ahead,
    previous_number: data.previous_number,
    next_number: data.next_number,
  };
}

type MyTicketsResponse = { action: 'list_my_tickets'; tickets: MyTicket[] };

/** Tiket saya di semua antrian, beserta nama dan status antriannya. */
export type MyTicket = Ticket & {
  queue_name: string;
  ticket_prefix?: string;
  queue_status: 'active' | 'closed';
  /** ticket_number yang sedang dilayani di antrian ini; null bila belum ada. */
  current_number: number | null;
};

/** Tiket yang masih relevan di beranda: menunggu di antrian aktif, atau nomornya sedang dilayani. */
export function isOnHome(t: MyTicket): boolean {
  if (t.queue_status !== 'active') return false;
  return t.status === 'waiting' || t.status === 'currently_served';
}

/** Tombol batal customer tampil hanya untuk nomor yang masih aktif di antrian yang masih buka (4.9). */
export function canCancelTicket(ticket: { status: string } | null | undefined, queueClosed: boolean): boolean {
  if (!ticket || queueClosed) return false;
  return ticket.status === 'waiting' || ticket.status === 'currently_served';
}

/**
 * POST /api/v1/queues/:id/cancel — customer membatalkan nomornya sendiri (4.9).
 * Tanpa tiket aktif, atau sudah dibatalkan: backend membalas tiket terakhir (no-op), bukan error.
 */
export async function cancelMyTicket(queueId: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/cancel`, { method: 'POST' });
}

/** GET /api/v1/users/me/tickets — tiket saya (login diperlukan). */
export async function listMyTickets(): Promise<MyTicket[]> {
  const data = await apiFetch<MyTicketsResponse>('/api/v1/users/me/tickets');
  return data.tickets;
}

