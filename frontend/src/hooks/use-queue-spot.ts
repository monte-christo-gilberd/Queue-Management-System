import { useCallback, useEffect, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { getQueueStatus } from '@/lib/tickets';
import type { QueueStatus } from '@/types/api';

const RELOAD_DEBOUNCE_MS = 300;

/** Status satu antrian dari sisi customer, diperbarui lewat SSE. `status` null selama muat awal. */
export function useQueueSpot(queueId: number, onChanged?: () => void) {
  const [status, setStatus] = useState<QueueStatus | null>(null);

  const reload = useCallback(async () => {
    try {
      setStatus(await getQueueStatus(queueId));
    } catch {
      // biarkan status terakhir; SSE akan mencoba lagi saat event berikutnya
    }
  }, [queueId]);

  // Muat awal. then() → setState tidak terjadi sinkron di dalam effect.
  useEffect(() => {
    let active = true;
    void getQueueStatus(queueId).then(
      (data) => {
        if (active) setStatus(data);
      },
      () => {},
    );
    return () => {
      active = false;
    };
  }, [queueId]);

  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const scheduleReload = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      void reload();
    }, RELOAD_DEBOUNCE_MS);
  }, [reload]);
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  useQueueEvents(queueId, {
    onEvent: (event) => {
      if (event.type === 'ticket_joined' || event.type === 'ticket_next' || event.type === 'ticket_skipped' || event.type === 'ticket_removed' || event.type === 'queue_closed') {
        scheduleReload();
        onChanged?.();
      }
    },
    onReconnect: scheduleReload,
    onResync: scheduleReload,
  });

  return status;
}
