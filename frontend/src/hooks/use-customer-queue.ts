import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { ApiError } from '@/lib/api';
import { describeQueueError } from '@/lib/queue-errors';
import { getQueueStatus, joinQueue } from '@/lib/tickets';
import type { QueueStatus } from '@/types/api';

const RELOAD_DEBOUNCE_MS = 300;

/**
 * Sisi customer untuk satu antrian: status tiket saya + gabung antrian + update lewat SSE.
 *
 * `status` null dan tanpa error = belum bergabung (backend membalas 404 untuk non-pemegang tiket).
 * Id tidak valid langsung dianggap tidak ditemukan tanpa request.
 */
export function useCustomerQueue(queueId: number) {
  const invalid = !Number.isInteger(queueId) || queueId <= 0;

  const [status, setStatus] = useState<QueueStatus | null>(null);
  const [loading, setLoading] = useState(!invalid);
  const [error, setError] = useState<string>();
  const [joining, setJoining] = useState(false);
  const [joinError, setJoinError] = useState<string>();

  const loadStatus = useCallback(async () => {
    try {
      const data = await getQueueStatus(queueId);
      setStatus(data);
      setError(undefined);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        setStatus(null);
        setError(undefined);
      } else {
        setError(describeQueueError(e));
      }
    } finally {
      setLoading(false);
    }
  }, [queueId]);

  // Muat awal. then() → setState tidak terjadi sinkron di dalam effect.
  useEffect(() => {
    if (invalid) return;
    let active = true;
    void getQueueStatus(queueId).then(
      (data) => {
        if (active) {
          setStatus(data);
          setError(undefined);
          setLoading(false);
        }
      },
      (e: unknown) => {
        if (!active) return;
        if (e instanceof ApiError && e.status === 404) {
          setStatus(null);
          setError(undefined);
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
    timerRef.current = setTimeout(() => {
      void loadStatus();
    }, RELOAD_DEBOUNCE_MS);
  }, [loadStatus]);
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  useQueueEvents(invalid ? null : queueId, {
    onEvent: (event) => {
      // Setiap perubahan antrian (tiket baru, nomor dilayani, ditutup) mengubah posisi saya.
      if (event.type === 'ticket_joined' || event.type === 'ticket_next' || event.type === 'queue_closed') {
        scheduleReload();
      }
    },
    onReconnect: () => {
      scheduleReload();
    },
  });

  const join = useCallback(async () => {
    setJoining(true);
    setJoinError(undefined);
    try {
      await joinQueue(queueId);
      await loadStatus();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'ALREADY_JOINED') {
        // sudah terdaftar (mis. klik ganda): cukup tampilkan statusnya
        await loadStatus();
      } else if (e instanceof ApiError && e.status === 404) {
        setJoinError('Antrian tidak ditemukan');
      } else {
        setJoinError(describeQueueError(e));
      }
    } finally {
      setJoining(false);
    }
  }, [queueId, loadStatus]);

  const invalidMessage = invalid ? 'Antrian tidak ditemukan' : undefined;

  return useMemo(
    () => ({
      status,
      loading,
      error: invalidMessage ?? error,
      joining,
      joinError,
      join,
    }),
    [status, loading, error, invalidMessage, joining, joinError, join],
  );
}
