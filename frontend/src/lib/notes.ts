import { apiFetch } from './api';

/** Catatan satu nomor tiket. note null bila nomor itu belum punya catatan. */
export type TicketNote = {
  ticket_id: number;
  note: string | null;
  created_at: string | null;
  updated_at: string | null;
};

/** GET /api/v1/queues/:id/tickets/:ticket_id/note — catatan nomor ini (owner). Tanpa catatan: note null. */
export async function getTicketNote(queueId: number, ticketId: number): Promise<TicketNote> {
  return apiFetch<TicketNote>(`/api/v1/queues/${queueId}/tickets/${ticketId}/note`);
}

/**
 * PUT /api/v1/queues/:id/tickets/:ticket_id/note — buat catatan (201) atau timpa yang ada (200).
 * Catatan kosong ditolak backend dengan 400; catatan lama tidak berubah.
 */
export async function saveTicketNote(queueId: number, ticketId: number, note: string): Promise<TicketNote> {
  return apiFetch<TicketNote>(`/api/v1/queues/${queueId}/tickets/${ticketId}/note`, {
    method: 'PUT',
    body: { note },
  });
}
