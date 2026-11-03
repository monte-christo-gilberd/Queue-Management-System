import { useCallback, useEffect, useState } from 'react';

import { getHistoryQueue, listHistoryQueues } from '@/lib/history';
import { describeQueueError } from '@/lib/queue-errors';
import type { HistoryQueue, HistoryTicket } from '@/types/api';

type Result<T> = { ok: true; data: T } | { ok: false; message: string };

async function run<T>(fn: () => Promise<T>): Promise<Result<T>> {
  try {
    return { ok: true, data: await fn() };
  } catch (e) {
    return { ok: false, message: describeQueueError(e) };
  }
}

/** Riwayat antrian owner (hanya yang sudah ditutup), dimuat per halaman. `loadMore` menambah halaman berikutnya. */
export function useHistoryQueues() {
  const [items, setItems] = useState<HistoryQueue[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string>();

  // Muat awal dengan then(), supaya setState tidak terjadi sinkron di dalam effect.
  useEffect(() => {
    let active = true;
    void run(() => listHistoryQueues()).then((r) => {
      if (!active) return;
      if (r.ok) {
        setItems(r.data.queues);
        setNextCursor(r.data.next_cursor);
        setError(undefined);
      } else {
        setError(r.message);
      }
      setLoading(false);
    });
    return () => {
      active = false;
    };
  }, []);

  const loadMore = useCallback(async () => {
    if (!nextCursor || loadingMore) return;
    setLoadingMore(true);
    const r = await run(() => listHistoryQueues(nextCursor));
    if (r.ok) {
      setItems((cur) => [...cur, ...r.data.queues]);
      setNextCursor(r.data.next_cursor);
      setError(undefined);
    } else {
      setError(r.message);
    }
    setLoadingMore(false);
  }, [nextCursor, loadingMore]);

  return { items, loading, loadingMore, hasMore: nextCursor !== null, loadMore, error };
}

/** Detail riwayat satu antrian yang sudah ditutup: ringkasan dan nomor-nomornya, per halaman. */
export function useHistoryQueue(queueId: number | null) {
  const [queue, setQueue] = useState<HistoryQueue | null>(null);
  const [tickets, setTickets] = useState<HistoryTicket[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(queueId !== null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (queueId === null) return;
    let active = true;
    void run(() => getHistoryQueue(queueId)).then((r) => {
      if (!active) return;
      if (r.ok) {
        setQueue(r.data.queue);
        setTickets(r.data.tickets);
        setNextCursor(r.data.next_cursor);
        setError(undefined);
      } else {
        setError(r.message);
      }
      setLoading(false);
    });
    return () => {
      active = false;
    };
  }, [queueId]);

  const loadMore = useCallback(async () => {
    if (queueId === null || !nextCursor || loadingMore) return;
    setLoadingMore(true);
    const r = await run(() => getHistoryQueue(queueId, nextCursor));
    if (r.ok) {
      setTickets((cur) => [...cur, ...r.data.tickets]);
      setNextCursor(r.data.next_cursor);
      setError(undefined);
    } else {
      setError(r.message);
    }
    setLoadingMore(false);
  }, [queueId, nextCursor, loadingMore]);

  return { queue, tickets, loading, loadingMore, hasMore: nextCursor !== null, loadMore, error };
}
