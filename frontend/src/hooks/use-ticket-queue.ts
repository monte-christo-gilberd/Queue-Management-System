import { useCallback, useEffect, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { ApiError } from '@/lib/api';
import { describeQueueError } from '@/lib/queue-errors';
import { getQueueStatus } from '@/lib/tickets';
import type { QueueStatus } from '@/types/api';

const RELOAD_DEBOUNCE_MS = 300;

/**
 * Status satu antrian untuk pemegang tiket (dibuka dari "Antrian Saya" / "Riwayat").
 * Tidak melakukan join: halaman join baru memakai token QR (`/join?t=...`).
 */
export function useTicketQueue(queueId: number) {
  const invalid = !Number.isInteger(queueId) || queueId <= 0;

  const [status, setStatus] = useState<QueueStatus | null>(null);
  const [loading, setLoading] = useState(!invalid);
  const [error, setError] = useState<string>();

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

  const hasTicket = status?.my_ticket != null;
  useQueueEvents(invalid || !hasTicket ? null : queueId, {
    onEvent: (event) => {
      if (
        event.type === 'ticket_joined' ||
        event.type === 'ticket_next' ||
        event.type === 'queue_closed' ||
        event.type === 'queue_updated'
      ) {
        scheduleReload();
      }
    },
    onReconnect: () => scheduleReload(),
  });

  return { status, loading, error };
}
