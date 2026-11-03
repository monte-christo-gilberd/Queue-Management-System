import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { describeQueueError } from '@/lib/queue-errors';
import { listTickets, nextTicket, recallTicket, removeTicket, skipTicket } from '@/lib/tickets';
import type { Ticket } from '@/types/api';
import { deriveQueueView } from '@/utils/queue-view';

/** Jeda sebelum mengambil ulang setelah event, supaya lonjakan join/next hanya memicu satu request. */
const RELOAD_DEBOUNCE_MS = 300;

type Result<T> = { ok: true; data: T } | { ok: false; message: string };

async function run<T>(fn: () => Promise<T>): Promise<Result<T>> {
  try {
    return { ok: true, data: await fn() };
  } catch (e) {
    return { ok: false, message: describeQueueError(e) };
  }
}

/**
 * Papan antrian satu queue: tiket dari server + pembaruan lewat SSE.
 *
 * Strategi: setiap event `ticket_joined` / `ticket_next` (dan setiap pemulihan koneksi) mengambil ulang
 * daftar tiket, alih-alih menerapkan delta. Ini sederhana dan tahan terhadap event yang terlewat.
 * `queue_closed` memanggil `onQueueClosed`.
 */
const RECALL_DELAY_MS = 3000;

export function useQueueBoard(queueId: number | null, onQueueClosed?: () => void) {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  const [loading, setLoading] = useState(queueId !== null);
  const [error, setError] = useState<string>();
  const [nextLoading, setNextLoading] = useState(false);
  const [nextError, setNextError] = useState<string>();
  const [skipLoading, setSkipLoading] = useState(false);
  const [skipError, setSkipError] = useState<string>();
  const [removeLoading, setRemoveLoading] = useState(false);
  const [removeError, setRemoveError] = useState<string>();
  const [recallLoading, setRecallLoading] = useState(false);
  const [recallError, setRecallError] = useState<string>();
  // Setelah Next/Skip: tombol Panggil ulang menggantikan Skip (mati 3 detik). Setelah dipanggil ulang,
  // Skip kembali (mati 3 detik). Backend tetap menegakkan aturan 3 detik dan sekali per nomor.
  const [recallReady, setRecallReady] = useState(true);
  const [showRecall, setShowRecall] = useState(false);
  const [skipReady, setSkipReady] = useState(true);
  const recallTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const armRecall = useCallback(() => {
    setShowRecall(true);
    setSkipReady(true);
    setRecallReady(false);
    if (recallTimer.current) clearTimeout(recallTimer.current);
    recallTimer.current = setTimeout(() => setRecallReady(true), RECALL_DELAY_MS);
  }, []);
  const armSkip = useCallback(() => {
    setShowRecall(false);
    setSkipReady(false);
    if (recallTimer.current) clearTimeout(recallTimer.current);
    recallTimer.current = setTimeout(() => setSkipReady(true), RECALL_DELAY_MS);
  }, []);
  useEffect(() => () => {
    if (recallTimer.current) clearTimeout(recallTimer.current);
  }, []);

  const apply = useCallback((r: Result<Ticket[]>) => {
    if (r.ok) {
      setTickets(r.data);
      setError(undefined);
    } else {
      setError(r.message);
    }
    setLoading(false);
  }, []);

  const reload = useCallback(
    async (id: number) => {
      apply(await run(() => listTickets(id)));
    },
    [apply],
  );

  // Muat awal saat queueId berubah. then() → setState tidak terjadi sinkron di dalam effect.
  useEffect(() => {
    if (queueId === null) return;
    let active = true;
    void run(() => listTickets(queueId)).then((r) => {
      if (active) apply(r);
    });
    return () => {
      active = false;
    };
  }, [apply, queueId]);

  // Debounce pengambilan ulang dari event SSE.
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const scheduleReload = useCallback(
    (id: number) => {
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
        void reload(id);
      }, RELOAD_DEBOUNCE_MS);
    },
    [reload],
  );
  useEffect(() => () => {
    if (timerRef.current) clearTimeout(timerRef.current);
  }, []);

  const closedRef = useRef(onQueueClosed);
  useEffect(() => {
    closedRef.current = onQueueClosed;
  });

  useQueueEvents(queueId, {
    onEvent: (event) => {
      if (
        event.type === 'ticket_joined' ||
        event.type === 'ticket_next' ||
        event.type === 'ticket_skipped' ||
        event.type === 'ticket_cancelled' ||
        event.type === 'ticket_removed'
      ) {
        scheduleReload(event.queue_id);
      } else if (event.type === 'queue_closed') {
        closedRef.current?.();
      }
    },
    onReconnect: () => {
      if (queueId !== null) scheduleReload(queueId);
    },
    onResync: () => {
      if (queueId !== null) scheduleReload(queueId);
    },
  });

  const next = useCallback(async (counterId = 0) => {
    if (queueId === null) return;
    setNextLoading(true);
    setNextError(undefined);
    const r = await run(() => nextTicket(queueId, counterId));
    if (r.ok) {
      armRecall();
      await reload(queueId);
    } else {
      setNextError(r.message);
    }
    setNextLoading(false);
  }, [queueId, reload, armRecall]);

  const skip = useCallback(async () => {
    if (queueId === null) return;
    setSkipLoading(true);
    setSkipError(undefined);
    const r = await run(() => skipTicket(queueId));
    if (r.ok) {
      armRecall();
      await reload(queueId);
    } else {
      setSkipError(r.message);
    }
    setSkipLoading(false);
  }, [queueId, reload, armRecall]);

  /** Hapus nomor customer dari antrian (4.10). Backend no-op bila nomor itu sudah tidak aktif. */
  const remove = useCallback(async (ticketId: number) => {
    if (queueId === null) return;
    setRemoveLoading(true);
    setRemoveError(undefined);
    const r = await run(() => removeTicket(queueId, ticketId));
    if (r.ok) {
      await reload(queueId);
    } else {
      setRemoveError(r.message);
    }
    setRemoveLoading(false);
  }, [queueId, reload]);

  const recall = useCallback(async () => {
    if (queueId === null) return;
    setRecallLoading(true);
    setRecallError(undefined);
    const r = await run(() => recallTicket(queueId));
    if (r.ok) {
      armSkip();
    } else {
      setRecallError(r.message);
    }
    setRecallLoading(false);
  }, [queueId, armSkip]);

  const view = useMemo(() => deriveQueueView(tickets), [tickets]);

  return {
    view,
    tickets,
    loading,
    error,
    next,
    nextLoading,
    nextError,
    skip,
    skipLoading,
    skipError,
    remove,
    removeLoading,
    removeError,
    recall,
    recallLoading,
    recallReady,
    showRecall,
    skipReady,
    recallError,
  };
}
