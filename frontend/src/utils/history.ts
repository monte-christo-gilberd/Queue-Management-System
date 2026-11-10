/**
 * history: label dan ringkasan riwayat antrian untuk owner dan customer. Fungsi murni.
 *
 * Dipakai oleh : src/components/History.tsx, HistoryDetail.tsx (owner); src/components/MyHistoryList.tsx (customer).
 * Bergantung   : types/api.ts (tipe riwayat), utils/format.ts (formatDateTime).
 * Catatan      : Ada dua set label hasil: owner (outcomeLabel) dan customer (customerOutcomeLabel).
 */

import type { HistoryOutcome, HistoryQueue, HistoryTicket, MyHistoryTicket } from '@/types/api';
import { formatDateTime } from '@/utils/format';

/** Label hasil untuk tampilan owner. */
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

const CUSTOMER_OUTCOME_LABEL: Record<HistoryOutcome, string> = {
  completed: 'Sudah dilayani',
  skipped: 'Dilewati',
  cancelled: 'Dibatalkan',
  deleted: 'Dihapus penyelenggara',
  unserved: 'Tidak sempat dilayani',
};

/** Label hasil akhir untuk customer (bahasa sisi customer, berbeda dengan label owner). */
export function customerOutcomeLabel(outcome: HistoryOutcome): string {
  return CUSTOMER_OUTCOME_LABEL[outcome];
}

/** Durasi dalam detik menjadi teks: "kurang dari 1 menit", "12 menit", "1 jam 5 menit", "2 jam". */
export function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60);
  if (minutes < 1) return 'kurang dari 1 menit';
  if (minutes < 60) return `${minutes} menit`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours} jam` : `${hours} jam ${rest} menit`;
}

/** Lama menunggu sampai dipanggil (bergabung ke dipanggil); null bila nomor belum pernah dipanggil. */
export function waitDuration(t: Pick<MyHistoryTicket, 'joined_at' | 'called_at'>): string | null {
  if (!t.called_at) return null;
  const seconds = (Date.parse(t.called_at) - Date.parse(t.joined_at)) / 1000;
  if (!Number.isFinite(seconds) || seconds < 0) return null;
  return formatDuration(seconds);
}
