import type { Ticket, TicketEntry } from '@/types/api';

/**
 * Tampilan antrian dari daftar tiket (`GET /queues/:id/tickets`). Fungsi murni.
 *
 * Aturan (sesuai perilaku `Next` di backend):
 * - `current`  = tiket `currently_served` milik counter `counterId` (multi-counter); tanpa counterId, yang paling baru dipanggil.
 * - `serving`  = semua tiket `currently_served`, satu per counter, yang paling baru dipanggil dulu.
 * - `previous` = tiket `completed` terbaru (completed_at paling besar).
 * - `next`     = tiket `waiting` pertama (sort_key terkecil).
 * - `upcoming` = sisa tiket `waiting` setelah `next`.
 * - `skipped`  = tiket `skipped` (dilewati), urut nomor.
 * Tiket `cancelled` dan `deleted` tidak ditampilkan.
 */
export type QueueView = {
  previous: TicketEntry | null;
  current: TicketEntry | null;
  serving: TicketEntry[];
  skipped: TicketEntry[];
  next: TicketEntry | null;
  upcoming: TicketEntry[];
};

function completedTime(ticket: Ticket): number {
  return ticket.completed_at ? Date.parse(ticket.completed_at) || 0 : 0;
}

export function deriveQueueView(tickets: readonly Ticket[], counterId = 0): QueueView {
  const completed = tickets
    .filter((t) => t.status === 'completed')
    .sort((a, b) => completedTime(b) - completedTime(a) || b.id - a.id);

  const servingAll = tickets
    .filter((t) => t.status === 'currently_served')
    .sort((a, b) => (Date.parse(b.called_at ?? '') || 0) - (Date.parse(a.called_at ?? '') || 0) || b.id - a.id);
  const serving =
    counterId > 0 ? servingAll.find((t) => t.called_counter_id === counterId) : servingAll[0];

  const waiting = tickets
    .filter((t) => t.status === 'waiting')
    .sort((a, b) => a.sort_key - b.sort_key || a.id - b.id);

  const skipped = tickets
    .filter((t) => t.status === 'skipped')
    .sort((a, b) => a.ticket_number - b.ticket_number);

  const entry = (ticket: Ticket): TicketEntry => ({ ticket, counterName: ticket.called_counter_name ?? undefined });

  return {
    previous: completed[0] ? entry(completed[0]) : null,
    current: serving ? entry(serving) : null,
    serving: servingAll.map(entry),
    skipped: skipped.map(entry),
    next: waiting[0] ? entry(waiting[0]) : null,
    upcoming: waiting.slice(1).map(entry),
  };
}
