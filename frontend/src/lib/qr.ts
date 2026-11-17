/**
 * qr.ts: API QR antrian untuk owner dan join antrian lewat token QR untuk customer.
 *
 * Dipakai oleh : hooks/use-queue-qr.ts (owner), components/ui/JoinQueueModals.tsx dan app/join.tsx (customer).
 * Bergantung   : lib/api.ts (apiFetch), lib/push.ts (enablePush, pushSupported).
 * Alur         : popup kamera: qr-scan.ts mengambil token -> joinByToken. Halaman /join?t=: previewJoin -> joinByToken.
 * Catatan      : kode galat QR dipetakan ke pesan di qr-errors.ts.
 */
import { apiFetch } from './api';
import { enablePush, pushSupported } from './push';
import type { PublicQueue, QueueQR, Ticket } from '@/types/api';

/**
 * GET /api/v1/queues/:id/qr — status QR untuk owner (static/dinamis + join_url).
 * Respons: `{ data: { action: "queue_qr", queue_id, type, join_url, expires_at } }`.
 */
export async function getQueueQR(queueId: number): Promise<QueueQR> {
  const data = await apiFetch<QueueQR>(`/api/v1/queues/${queueId}/qr`);
  return data;
}

/**
 * PUT /api/v1/queues/:id/qr/type — set mode QR antrian (static | dynamic).
 * Respons: `{ data: QueueQR }` (untuk dynamic membawa expires_at).
 */
export async function setQueueQRType(queueId: number, type: 'static' | 'dynamic'): Promise<QueueQR> {
  const data = await apiFetch<QueueQR>(`/api/v1/queues/${queueId}/qr/type`, {
    method: 'PUT',
    body: { type },
  });
  return data;
}

/**
 * POST /api/v1/queues/:id/qr/replace — mengganti kode statis dengan kode baru.
 * Hanya untuk type static.
 * Respons: `{ data: QueueQR }` (type selalu "static" setelah penggantian).
 */
export async function replaceQueueQR(queueId: number): Promise<QueueQR> {
  const data = await apiFetch<QueueQR>(`/api/v1/queues/${queueId}/qr/replace`, { method: 'POST' });
  return data;
}

/**
 * GET /api/v1/join/preview?t=<token> — info publik antrian dari token join (login diperlukan).
 * Token salah/expired -> error (QR_INVALID/QR_EXPIRED/QR_UNAVAILABLE).
 */
export async function previewJoin(token: string): Promise<PublicQueue> {
  const data = await apiFetch<{ action: 'join_preview'; queue: PublicQueue }>(
    `/api/v1/join/preview?t=${encodeURIComponent(token)}`,
  );
  return data.queue;
}

/**
 * POST /api/v1/join {t} — customer bergabung ke antrian lewat token join.
 * Token valid -> tiket. 409 ALREADY_JOINED / QUEUE_CLOSED, 404 QR_INVALID,
 * 410 QR_EXPIRED, 503 QR_UNAVAILABLE.
 */
export async function joinByToken(token: string): Promise<Ticket> {
  const ticket = await apiFetch<Ticket>('/api/v1/join', { method: 'POST', body: { t: token } });
  // Aktifkan notifikasi otomatis; bila izin ditolak atau tidak didukung, tidak terjadi apa-apa.
  if (pushSupported() && Notification.permission !== 'denied') {
    void enablePush().catch(() => undefined);
  }
  return ticket;
}
