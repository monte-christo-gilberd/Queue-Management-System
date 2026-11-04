import { useCallback, useEffect, useState } from 'react';

import { ApiError } from '@/lib/api';
import {
  downloadHistoryExport,
  getHistoryQueue,
  listHistoryQueues,
  listMyHistory,
  type ExportFormat,
} from '@/lib/history';
import { describeQueueError } from '@/lib/queue-errors';
import type { HistoryQueue, HistoryTicket, MyHistoryTicket } from '@/types/api';

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
  const [exporting, setExporting] = useState<ExportFormat | null>(null);
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

  /** Unduh seluruh nomor antrian sebagai CSV atau Excel. Galat (termasuk native yang belum didukung) masuk `error`. */
  const exportAs = useCallback(
    async (format: ExportFormat) => {
      if (!queue || exporting) return;
      setExporting(format);
      try {
        await downloadHistoryExport(queue, format);
        setError(undefined);
      } catch (e) {
        setError(e instanceof Error && !(e instanceof ApiError) ? e.message : describeQueueError(e));
      } finally {
        setExporting(null);
      }
    },
    [queue, exporting],
  );

  return { queue, tickets, loading, loadingMore, hasMore: nextCursor !== null, loadMore, exporting, exportAs, error };
}

/** Riwayat tiket saya (customer), satu baris per tiket, dimuat per halaman. `loadMore` menambah halaman berikutnya. */
export function useMyHistory() {
  const [items, setItems] = useState<MyHistoryTicket[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    let active = true;
    void run(() => listMyHistory()).then((r) => {
      if (!active) return;
      if (r.ok) {
        setItems(r.data.tickets);
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
    const r = await run(() => listMyHistory(nextCursor));
    if (r.ok) {
      setItems((cur) => [...cur, ...r.data.tickets]);
      setNextCursor(r.data.next_cursor);
      setError(undefined);
    } else {
      setError(r.message);
    }
    setLoadingMore(false);
  }, [nextCursor, loadingMore]);

  return { items, loading, loadingMore, hasMore: nextCursor !== null, loadMore, error };
}
