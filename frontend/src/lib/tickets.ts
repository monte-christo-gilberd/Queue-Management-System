/**
 * tickets.ts: panggilan API tiket untuk owner (panggil, lewati, panggil ulang, sisip, hapus) dan customer
 * (gabung, batal, daftar tiket saya), beserta aturan tombol yang bergantung pada status tiket.
 *
 * Dipakai oleh : hooks/use-queue-board.ts, hooks/use-ticket-queue.ts, hooks/use-queue-spot.ts, hooks/use-my-tickets.ts,
 *                components/MyTicketsList.tsx, components/ui/InsertTicketModal.tsx, app/(user)/antrian.tsx,
 *                app/ticket/[queueId].tsx
 * Bergantung   : lib/api.ts (apiFetch), types/api.ts (Ticket, PublicQueue, QueueStatus)
 * Catatan      : Galat 409 (QUEUE_EMPTY, NOTHING_TO_SKIP, QUEUE_CLOSED, dll.) dibaca dari ApiError.code, bukan dari status saja.
 */
import { apiFetch } from './api';
import type { QueueStatus, Ticket } from '@/types/api';

/** Bentuk `data` dari GET /api/v1/queues/:id/tickets. */
type ListTicketsResponse = {
  /** Penanda respons dari backend. */
  action: 'list_tickets';
  /** ID antrian yang daftar tiketnya dikembalikan. */
  queue_id: number;
  /** Tiket antrian, urut sort_key. */
  tickets: Ticket[];
};

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
export async function nextTicket(queueId: number, counterId = 0, expectedTicketId?: number): Promise<Ticket> {
  // counter_id 0 = counter default milik queue. expected_ticket_id: nomor yang dilihat counter ini (D9); 0 = kosong.
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/next`, {
    method: 'POST',
    body: { counter_id: counterId, expected_ticket_id: expectedTicketId },
  });
}

/** POST /api/v1/queues/:id/recall — owner memanggil ulang nomor yang sedang dilayani (push saja). */
export async function recallTicket(queueId: number, counterId = 0, expectedTicketId?: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/recall`, {
    method: 'POST',
    body: { counter_id: counterId, expected_ticket_id: expectedTicketId },
  });
}

/**
 * POST /api/v1/queues/:id/skip — lewati nomor yang sedang dilayani dan panggil berikutnya (owner).
 * Mengembalikan tiket yang baru dipanggil. 409 NOTHING_TO_SKIP bila belum ada yang dilayani,
 * 409 QUEUE_EMPTY bila tidak ada yang menunggu (nomor tetap dilayani).
 */
export async function skipTicket(queueId: number, counterId = 0, expectedTicketId?: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/skip`, {
    method: 'POST',
    body: { counter_id: counterId, expected_ticket_id: expectedTicketId },
  });
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
    serving: data.serving,
    waiting_ahead: data.waiting_ahead,
    previous_number: data.previous_number,
    next_number: data.next_number,
  };
}

/** Bentuk `data` dari GET /api/v1/users/me/tickets. */
type MyTicketsResponse = {
  /** Penanda respons dari backend. */
  action: 'list_my_tickets';
  /** Tiket milik user yang login, di semua antrian. */
  tickets: MyTicket[];
};

/** MyTicket: tiket saya di semua antrian, beserta nama dan status antriannya. */
export type MyTicket = Ticket & {
  /** Nama antrian tempat tiket ini berada. */
  queue_name: string;
  /** Prefix nomor antrian (mis. A). Opsional. */
  ticket_prefix?: string;
  /** Status antrian: 'active' bila masih buka, 'closed' bila sudah ditutup. */
  queue_status: 'active' | 'closed';
  /** ticket_number yang sedang dilayani di antrian ini; null bila belum ada. */
  current_number: number | null;
};

/**
 * isOnHome: true bila tiket masih relevan di beranda, yaitu antrian masih aktif dan tiket berstatus
 * waiting atau currently_served.
 */
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

/**
 * validateInsertInput: isian sisip (US-10). Identifier wajib, posisi bilangan bulat minimal 1.
 * Mengembalikan pesan galat, atau null bila valid. Contoh: validateInsertInput('', 1) = 'Isi email atau username customer'.
 */
export function validateInsertInput(identifier: string, position: number): string | null {
  if (identifier.trim() === '') return 'Isi email atau username customer';
  if (!Number.isInteger(position) || position < 1) return 'Posisi harus bilangan bulat, minimal 1';
  return null;
}

/**
 * POST /api/v1/queues/:id/insert — owner menyisipkan akun terdaftar di posisi tertentu (US-10).
 * Posisi dihitung dari tiket waiting: 1 = berikutnya. Posisi melebihi panjang antrian masuk di paling belakang.
 */
export async function insertTicket(queueId: number, identifier: string, position: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/insert`, {
    method: 'POST',
    body: { identifier: identifier.trim(), position },
  });
}

/** Owner boleh menghapus nomor yang masih aktif di antrian (waiting atau currently_served), 4.10. */
export function canRemoveTicket(ticket: { status: string }): boolean {
  return ticket.status === 'waiting' || ticket.status === 'currently_served';
}

/**
 * POST /api/v1/queues/:id/tickets/:ticket_id/remove — owner menghapus nomor customer (4.10).
 * Status menjadi `deleted`, tanpa fallback; nomor yang sedang dilayani membuat nomor berikutnya naik
 * di counter yang sama.
 */
export async function removeTicket(queueId: number, ticketId: number): Promise<Ticket> {
  return apiFetch<Ticket>(`/api/v1/queues/${queueId}/tickets/${ticketId}/remove`, { method: 'POST' });
}

/** GET /api/v1/users/me/tickets — tiket saya (login diperlukan). */
export async function listMyTickets(): Promise<MyTicket[]> {
  const data = await apiFetch<MyTicketsResponse>('/api/v1/users/me/tickets');
  return data.tickets;
}
