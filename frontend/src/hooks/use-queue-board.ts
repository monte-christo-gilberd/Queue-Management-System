import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { ApiError } from '@/lib/api';
import { describeInsertError, describeQueueError } from '@/lib/queue-errors';
import { insertTicket, listTickets, nextTicket, recallTicket, skipTicket } from '@/lib/tickets';
import type { Ticket } from '@/types/api';
import { deriveQueueView } from '@/utils/queue-view';

/** Jeda sebelum mengambil ulang setelah event, supaya lonjakan join/next hanya memicu satu request. */
const RELOAD_DEBOUNCE_MS = 300;

type Result<T> = { ok: true; data: T } | { ok: false; message: string; code?: string };

async function run<T>(fn: () => Promise<T>): Promise<Result<T>> {
  try {
    return { ok: true, data: await fn() };
  } catch (e) {
    return { ok: false, message: describeQueueError(e), code: e instanceof ApiError ? e.code : undefined };
  }
}

/** Kode 409 dari server: counter ini sudah berubah di perangkat lain sejak papan dimuat. */
const COUNTER_CHANGED = 'COUNTER_STATE_CHANGED';

/**
 * Papan antrian satu queue: tiket dari server + pembaruan lewat SSE.
 *
 * Strategi: setiap event `ticket_joined` / `ticket_next` (dan setiap pemulihan koneksi) mengambil ulang
 * daftar tiket, alih-alih menerapkan delta. Ini sederhana dan tahan terhadap event yang terlewat.
 * `queue_closed` memanggil `onQueueClosed`.
 */
const RECALL_DELAY_MS = 3000;

export function useQueueBoard(queueId: number | null, onQueueClosed?: () => void, counterId = 0) {
  const [tickets, setTickets] = useState<Ticket[]>([]);
  // Pemberitahuan (bukan galat) bila counter ini dilanjutkan dari perangkat lain; papan sudah dimuat ulang.
  const [syncNotice, setSyncNotice] = useState<string>();
  const [loading, setLoading] = useState(queueId !== null);
  const [error, setError] = useState<string>();
  const [nextLoading, setNextLoading] = useState(false);
  const [nextError, setNextError] = useState<string>();
  const [skipLoading, setSkipLoading] = useState(false);
  const [skipError, setSkipError] = useState<string>();
  const [insertLoading, setInsertLoading] = useState(false);
  const [insertError, setInsertError] = useState<string>();
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
        event.type === 'counter_updated'
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

  /** Nomor yang sedang dilayani counter ini menurut tampilan perangkat; 0 bila counter kosong. */
  const expectedServing = useCallback(
    () => deriveQueueView(tickets, counterId).current?.ticket.id ?? 0,
    [tickets, counterId],
  );

  /** Menangani hasil aksi counter: 409 COUNTER_STATE_CHANGED memuat ulang diam-diam dan memberi pemberitahuan. */
  const settleCounter = useCallback(
    async (r: Result<unknown>, onOk: () => void, onError: (message: string) => void) => {
      if (queueId === null) return;
      if (r.ok) {
        setSyncNotice(undefined);
        onOk();
        await reload(queueId);
      } else if (r.code === COUNTER_CHANGED) {
        setSyncNotice(r.message);
        await reload(queueId);
      } else {
        onError(r.message);
      }
    },
    [queueId, reload],
  );

  const next = useCallback(async () => {
    if (queueId === null) return;
    setNextLoading(true);
    setNextError(undefined);
    const r = await run(() => nextTicket(queueId, counterId, expectedServing()));
    await settleCounter(r, armRecall, setNextError);
    setNextLoading(false);
  }, [queueId, counterId, expectedServing, settleCounter, armRecall]);

  const skip = useCallback(async () => {
    if (queueId === null) return;
    setSkipLoading(true);
    setSkipError(undefined);
    const r = await run(() => skipTicket(queueId, counterId, expectedServing()));
    await settleCounter(r, armRecall, setSkipError);
    setSkipLoading(false);
  }, [queueId, counterId, expectedServing, settleCounter, armRecall]);

  /**
   * Sisip customer ke posisi tertentu (US-10). Mengembalikan true bila berhasil, agar modal tahu kapan menutup diri.
   * Galat ditampilkan dengan kalimat khusus sisip (describeInsertError), bukan pesan aksi antrian umum.
   */
  const insert = useCallback(async (identifier: string, position: number): Promise<boolean> => {
    if (queueId === null) return false;
    setInsertLoading(true);
    setInsertError(undefined);
    try {
      await insertTicket(queueId, identifier, position);
      await reload(queueId);
      return true;
    } catch (e) {
      setInsertError(describeInsertError(e));
      return false;
    } finally {
      setInsertLoading(false);
    }
  }, [queueId, reload]);

  const recall = useCallback(async () => {
    if (queueId === null) return;
    setRecallLoading(true);
    setRecallError(undefined);
    const r = await run(() => recallTicket(queueId, counterId, expectedServing()));
    await settleCounter(r, armSkip, setRecallError);
    setRecallLoading(false);
  }, [queueId, counterId, expectedServing, settleCounter, armSkip]);

  const view = useMemo(() => deriveQueueView(tickets, counterId), [tickets, counterId]);

  return {
    view,
    tickets,
    counterId,
    syncNotice,
    loading,
    error,
    next,
    nextLoading,
    nextError,
    skip,
    skipLoading,
    skipError,
    recall,
    insert,
    insertLoading,
    insertError,
    recallLoading,
    recallReady,
    showRecall,
    skipReady,
    recallError,
  };
}
