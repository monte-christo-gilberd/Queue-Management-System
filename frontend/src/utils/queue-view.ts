/**
 * queue-view: aturan tampilan antrian owner (sedang dilayani, berikutnya, dilewati) dari daftar tiket.
 *
 * Dipakai oleh : src/hooks/use-queue-board.ts (deriveQueueView, dengan counterId counter yang dipilih perangkat).
 * Bergantung   : types/api.ts (Ticket, TicketEntry).
 * Alur         : GET /queues/:id/tickets -> use-queue-board.ts -> deriveQueueView -> layar owner.
 * Catatan      : Aturan ini mengikuti perilaku `Next` di backend; tidak ada pemanggilan API di sini.
 */

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
  /** Nomor terakhir yang selesai dilayani; null bila belum ada. */
  previous: TicketEntry | null;
  /** Nomor yang sedang dilayani counter yang dipilih (atau yang paling baru dipanggil); null bila tidak ada. */
  current: TicketEntry | null;
  /** Semua nomor yang sedang dilayani, satu per counter, yang paling baru dipanggil dulu. */
  serving: TicketEntry[];
  /** Nomor yang dilewati, urut nomor. */
  skipped: TicketEntry[];
  /** Nomor menunggu berikutnya; null bila tidak ada yang menunggu. */
  next: TicketEntry | null;
  /** Sisa nomor menunggu setelah `next`, urut antrean. */
  upcoming: TicketEntry[];
};

/** Waktu selesai dalam milidetik; 0 bila completed_at kosong atau tidak valid, sehingga jatuh ke urutan paling belakang. */
function completedTime(ticket: Ticket): number {
  return ticket.completed_at ? Date.parse(ticket.completed_at) || 0 : 0;
}

/**
 * deriveQueueView: menurunkan QueueView dari daftar tiket. `counterId` 0 berarti tanpa filter counter.
 * Contoh: tiket waiting nomor 5 (sort_key 2) dan nomor 6 (sort_key 1) menghasilkan next = nomor 6.
 */
export function deriveQueueView(tickets: readonly Ticket[], counterId = 0): QueueView {
  // Selesai terbaru dulu; bila waktunya sama, id yang lebih besar didahulukan.
  const completed = tickets
    .filter((t) => t.status === 'completed')
    .sort((a, b) => completedTime(b) - completedTime(a) || b.id - a.id);

  // Yang paling baru dipanggil di urutan pertama. Dengan counterId, current diambil dari counter itu saja.
  const servingAll = tickets
    .filter((t) => t.status === 'currently_served')
    .sort((a, b) => (Date.parse(b.called_at ?? '') || 0) - (Date.parse(a.called_at ?? '') || 0) || b.id - a.id);
  const serving =
    counterId > 0 ? servingAll.find((t) => t.called_counter_id === counterId) : servingAll[0];

  // sort_key menentukan urutan antre; id hanya sebagai pengikat agar urutannya stabil.
  const waiting = tickets
    .filter((t) => t.status === 'waiting')
    .sort((a, b) => a.sort_key - b.sort_key || a.id - b.id);

  const skipped = tickets
    .filter((t) => t.status === 'skipped')
    .sort((a, b) => a.ticket_number - b.ticket_number);

  // counterName diambil dari tiket sendiri; hanya terisi bila tiket dipanggil oleh counter bernama.
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
