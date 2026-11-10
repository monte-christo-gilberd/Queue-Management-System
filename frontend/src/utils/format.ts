/**
 * format: pemformat nomor tiket, tanggal, dan waktu untuk tampilan berbahasa Indonesia. Fungsi murni.
 *
 * Dipakai oleh : src/utils/history.ts, src/utils/profile.ts, src/components/*.tsx, src/app/* (antrian dan tiket).
 * Catatan      : Tanggal dan jam memakai zona waktu perangkat (Date lokal). Contoh di komentar berlaku bila
 *                zona perangkat UTC.
 */

/** Singkatan nama bulan Bahasa Indonesia; indeks 0 = Januari (sesuai Date.getMonth). */
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'];

/** Format tampilan nomor tiket, mis. 42 -> "A-042". Prefix dari antrian (`ticket_prefix`), default "A". */
export function formatTicketNumber(ticketNumber: number, prefix?: string | null): string {
  return `${prefix || 'A'}-${String(ticketNumber).padStart(3, '0')}`;
}

/** Angka dua digit dengan nol di depan, mis. 7 -> "07". */
function pad2(value: number): string {
  return String(value).padStart(2, '0');
}

/** "2025-01-12T08:30:00Z" -> "12 Jan 2025" */
export function formatDate(iso: string): string {
  const d = new Date(iso);
  return `${d.getDate()} ${MONTHS[d.getMonth()]} ${d.getFullYear()}`;
}

/** "2025-01-12T08:30:00Z" -> "12 Jan 2025 · 08:30" */
export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  return `${formatDate(iso)} · ${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}
