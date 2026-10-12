const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'Mei', 'Jun', 'Jul', 'Agu', 'Sep', 'Okt', 'Nov', 'Des'];

/** Format tampilan nomor tiket, mis. 42 -> "A-042". */
export function formatTicketNumber(ticketNumber: number): string {
  // TODO: konfirmasi sumber data — prefix huruf ("A-") belum ada di backend.
  return `A-${String(ticketNumber).padStart(3, '0')}`;
}

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
