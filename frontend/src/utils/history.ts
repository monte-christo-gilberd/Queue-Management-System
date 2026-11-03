import type { HistoryOutcome, HistoryQueue, HistoryTicket } from '@/types/api';
import { formatDateTime } from '@/utils/format';

const OUTCOME_LABEL: Record<HistoryOutcome, string> = {
  completed: 'Selesai',
  skipped: 'Dilewati',
  cancelled: 'Dibatalkan',
  deleted: 'Dihapus',
  unserved: 'Belum dilayani',
};

/** Label hasil akhir sebuah nomor. */
export function outcomeLabel(outcome: HistoryOutcome): string {
  return OUTCOME_LABEL[outcome];
}

/**
 * Ringkasan satu baris untuk kartu riwayat, mis. "12 nomor · 9 selesai · 2 dilewati · 1 dibatalkan".
 * Hasil yang bernilai 0 tidak ditampilkan; antrian tanpa tiket menjadi "Tidak ada nomor".
 */
export function summarizeHistoryQueue(q: HistoryQueue): string {
  if (q.total === 0) return 'Tidak ada nomor';
  const parts = [`${q.total} nomor`];
  const counts: [number, string][] = [
    [q.completed, 'selesai'],
    [q.skipped, 'dilewati'],
    [q.cancelled, 'dibatalkan'],
    [q.deleted, 'dihapus'],
    [q.unserved, 'belum dilayani'],
  ];
  for (const [n, label] of counts) if (n > 0) parts.push(`${n} ${label}`);
  return parts.join(' · ');
}

/** Baris waktu untuk satu nomor: kapan bergabung, lalu kapan selesai atau dibatalkan bila ada. */
export function ticketTimeline(t: HistoryTicket): string[] {
  const lines = [`Bergabung ${formatDateTime(t.joined_at)}`];
  if (t.completed_at) lines.push(`Selesai ${formatDateTime(t.completed_at)}`);
  if (t.cancelled_at) lines.push(`Dibatalkan ${formatDateTime(t.cancelled_at)}`);
  return lines;
}
