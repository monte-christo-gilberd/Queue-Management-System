/**
 * notes.ts: API catatan per nomor tiket untuk owner.
 *
 * Dipakai oleh : hooks/use-ticket-note.ts, components/QueueDetail.tsx.
 * Bergantung   : lib/api.ts (apiFetch).
 */
import { apiFetch } from './api';

/** TicketNote: catatan satu nomor tiket. `note` bernilai null bila nomor itu belum punya catatan. */
export type TicketNote = {
  /** ID nomor tiket di dalam antrian. */
  ticket_id: number;
  /** Teks catatan; null bila belum ada catatan untuk nomor ini. */
  note: string | null;
  /** Waktu catatan dibuat (string ISO); null bila belum ada catatan. */
  created_at: string | null;
  /** Waktu catatan terakhir diubah (string ISO); null bila belum ada catatan. */
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
