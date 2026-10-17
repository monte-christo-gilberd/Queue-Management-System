import type { Ticket, TicketEntry } from '@/types/api';

/**
 * Tampilan antrian dari daftar tiket (`GET /queues/:id/tickets`). Fungsi murni.
 *
 * Aturan (sesuai perilaku `Next` di backend):
 * - `current`  = tiket `completed` terbaru (completed_at paling besar).
 * - `previous` = tiket `completed` sebelumnya.
 * - `next`     = tiket `waiting` pertama (sort_key terkecil).
 * - `upcoming` = sisa tiket `waiting` setelah `next`.
 * Tiket `skipped` dan `cancelled` tidak ditampilkan.
 */
export type QueueView = {
  previous: TicketEntry | null;
  current: TicketEntry | null;
  next: TicketEntry | null;
  upcoming: TicketEntry[];
};

function completedTime(ticket: Ticket): number {
  return ticket.completed_at ? Date.parse(ticket.completed_at) || 0 : 0;
}

export function deriveQueueView(tickets: readonly Ticket[]): QueueView {
  const completed = tickets
    .filter((t) => t.status === 'completed')
    .sort((a, b) => completedTime(b) - completedTime(a) || b.id - a.id);

  const waiting = tickets
    .filter((t) => t.status === 'waiting')
    .sort((a, b) => a.sort_key - b.sort_key || a.id - b.id);

  const entry = (ticket: Ticket): TicketEntry => ({ ticket });

  return {
    previous: completed[1] ? entry(completed[1]) : null,
    current: completed[0] ? entry(completed[0]) : null,
    next: waiting[0] ? entry(waiting[0]) : null,
    upcoming: waiting.slice(1).map(entry),
  };
}
