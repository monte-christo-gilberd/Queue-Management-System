/**
 * qr-errors.ts: pesan untuk galat terkait QR.
 *
 * Dipakai oleh : lib/queue-errors.ts (untuk kode berawalan `QR_`), hooks/use-queue-qr.ts.
 * Catatan      : kode yang tidak ada di peta jatuh ke QR_DEFAULT_MESSAGE, kecuali NETWORK_ERROR yang memakai pesan asli.
 *                NOT_FOUND dan SUBSCRIPTION_INACTIVE di peta hanya terpakai bila describeQrError dipanggil langsung,
 *                karena queue-errors.ts menangani kode non-QR sendiri.
 */
import { ApiError } from './api';

/** Satu peta kode error QR (backend) ke pesan. Dipakai layar owner dan customer. */
const QR_MESSAGES: Record<string, string> = {
  QR_INVALID: 'This QR link is invalid or no longer valid. Scan the latest QR from the queue owner.',
  QR_EXPIRED: 'This QR link has expired. Scan the latest QR again.',
  QR_UNAVAILABLE: 'The QR service is unavailable right now. Please try again.',
  QR_TYPE_INVALID: 'Unknown QR mode. Choose static or dynamic.',
  QR_REPLACE_NOT_ALLOWED: 'A dynamic QR cannot be replaced. Let it expire; it refreshes automatically.',
  NOT_FOUND: 'Queue not found.',
  SUBSCRIPTION_INACTIVE: 'Your subscription is inactive. Activate it to manage queues.',
};

/** QR_DEFAULT_MESSAGE: pesan umum untuk galat QR yang tidak dikenal atau bukan ApiError. */
export const QR_DEFAULT_MESSAGE = 'Something went wrong with the QR code. Please try again.';

/**
 * isQrErrorCode mengembalikan true bila kode error berawalan `QR_`.
 * Contoh: `QR_EXPIRED` menghasilkan true; `NOT_FOUND` menghasilkan false.
 */
export function isQrErrorCode(code: string): boolean {
  return code.startsWith('QR_');
}

/** Pesan QR untuk error apa pun; kode tak dikenal jatuh ke pesan umum. */
export function describeQrError(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === 'NETWORK_ERROR') return error.message;
    return QR_MESSAGES[error.code] ?? QR_DEFAULT_MESSAGE;
  }
  return QR_DEFAULT_MESSAGE;
}
