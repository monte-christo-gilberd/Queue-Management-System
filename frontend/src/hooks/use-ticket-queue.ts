/**
 * use-ticket-queue: status satu antrian untuk pemegang tiket, dibuka dari "Antrian Saya" atau "Riwayat".
 *
 * Dipakai oleh : src/app/ticket/[queueId].tsx.
 * Bergantung   : hooks/use-queue-events (SSE), lib/api (ApiError), lib/queue-errors (describeQueueError), lib/tickets (getQueueStatus, cancelMyTicket), types/api (QueueStatus).
 * Alur         : queueId -> getQueueStatus -> status. SSE dibuka hanya bila akun punya tiket di antrian ini. Event yang relevan memicu muat ulang (debounce 300 ms).
 * Catatan      : Hook ini tidak melakukan join. Join memakai token QR di halaman /join. Cancel memuat ulang status setelah berhasil.
 */

import { useCallback, useEffect, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { ApiError } from '@/lib/api';
import { describeQueueError } from '@/lib/queue-errors';
import { cancelMyTicket, getQueueStatus } from '@/lib/tickets';
import type { QueueStatus } from '@/types/api';

const RELOAD_DEBOUNCE_MS = 300;

/**
 * Status satu antrian untuk pemegang tiket (dibuka dari "Antrian Saya" / "Riwayat").
 * Tidak melakukan join: halaman join baru memakai token QR (`/join?t=...`).
 *
 * Argumen: `queueId` adalah id antrian. Bila bukan bilangan bulat positif, hook tidak membuat request dan `loading` false.
 *
 * Mengembalikan objek dengan field:
 * - `status`: status antrian dan nomor akun ini (`my_ticket`). null bila belum dimuat atau antrian tidak ditemukan.
 * - `loading`: true selama muat awal.
 * - `error`: pesan galat muat. "Antrian tidak ditemukan" untuk 404.
 * - `cancel()`: membatalkan nomor akun ini di antrian.
 * - `cancelling`: true selama request batal berjalan.
 * - `cancelError`: pesan galat dari batal. undefined bila tidak ada.
 */
export function useTicketQueue(queueId: number) {
  const invalid = !Number.isInteger(queueId) || queueId <= 0;

  const [status, setStatus] = useState<QueueStatus | null>(null);
  const [loading, setLoading] = useState(!invalid);
  const [error, setError] = useState<string>();

  // Muat status untuk dipakai ulang oleh effect, debounce SSE, dan cancel. 404 membedakan antrian yang hilang dari galat jaringan.
  const loadStatus = useCallback(async () => {
    if (invalid) return;
    try {
      setStatus(await getQueueStatus(queueId));
      setError(undefined);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        setStatus(null);
        setError('Antrian tidak ditemukan');
      } else {
        setError(describeQueueError(e));
      }
    } finally {
      setLoading(false);
    }
  }, [invalid, queueId]);

  // Muat awal dengan `.then()` agar setState tidak sinkron di dalam effect.
  useEffect(() => {
    if (invalid) return;
    let active = true;
    void getQueueStatus(queueId).then(
      (data) => {
        if (!active) return;
        setStatus(data);
        setError(undefined);
        setLoading(false);
      },
      (e: unknown) => {
        if (!active) return;
        if (e instanceof ApiError && e.status === 404) {
          setStatus(null);
          setError('Antrian tidak ditemukan');
        } else {
          setError(describeQueueError(e));
        }
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
  }, [invalid, queueId]);

  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const scheduleReload = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => void loadStatus(), RELOAD_DEBOUNCE_MS);
  }, [loadStatus]);
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  // SSE hanya dibuka bila akun punya tiket di antrian ini. Tanpa tiket, tidak ada alasan menahan koneksi.
  const hasTicket = status?.my_ticket != null;
  useQueueEvents(invalid || !hasTicket ? null : queueId, {
    onEvent: (event) => {
      if (
        event.type === 'ticket_joined' ||
        event.type === 'ticket_next' ||
        event.type === 'ticket_skipped' ||
        event.type === 'ticket_cancelled' ||
        event.type === 'ticket_removed' ||
        event.type === 'queue_closed' ||
        event.type === 'queue_updated'
      ) {
        scheduleReload();
      }
    },
    onReconnect: () => scheduleReload(),
    onResync: () => scheduleReload(),
  });

  const [cancelling, setCancelling] = useState(false);
  const [cancelError, setCancelError] = useState<string>();

  /** Batalkan nomor saya (4.9). Backend no-op bila sudah tidak aktif, jadi klik ganda aman. */
  const cancel = useCallback(async () => {
    if (invalid) return;
    setCancelling(true);
    setCancelError(undefined);
    try {
      await cancelMyTicket(queueId);
      await loadStatus();
    } catch (e) {
      setCancelError(describeQueueError(e));
    } finally {
      setCancelling(false);
    }
  }, [invalid, queueId, loadStatus]);

  return { status, loading, error, cancel, cancelling, cancelError };
}
