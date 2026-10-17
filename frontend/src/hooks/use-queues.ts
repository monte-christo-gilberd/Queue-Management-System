import { useCallback, useEffect, useState } from 'react';

import { describeQueueError } from '@/lib/queue-errors';
import { getQueue, listQueues } from '@/lib/queues';
import type { Queue, QueueAuthz } from '@/types/api';

type Result<T> = { ok: true; data: T } | { ok: false; message: string };

/** Menjalankan request dan mengubah error menjadi pesan. Tidak menyentuh state React. */
async function run<T>(fn: () => Promise<T>): Promise<Result<T>> {
  try {
    return { ok: true, data: await fn() };
  } catch (e) {
    return { ok: false, message: describeQueueError(e) };
  }
}

/**
 * Daftar antrian owner. `reload` dipakai setelah buat/tutup antrian.
 * Effect hanya memanggil `.then()`, sehingga setState tidak terjadi sinkron di dalam effect.
 */
export function useQueueList() {
  const [queues, setQueues] = useState<Queue[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const apply = useCallback((r: Result<Queue[]>) => {
    if (r.ok) {
      setQueues(r.data);
      setError(undefined);
    } else {
      setError(r.message);
    }
    setLoading(false);
  }, []);

  const reload = useCallback(async () => {
    apply(await run(listQueues));
  }, [apply]);

  useEffect(() => {
    let active = true;
    void run(listQueues).then((r) => {
      if (active) apply(r);
    });
    return () => {
      active = false;
    };
  }, [apply]);

  return { queues, loading, error, reload };
}

/** Detail satu antrian beserta izinnya. Id tidak valid langsung dianggap tidak ditemukan (tanpa request). */
export function useQueueDetail(queueId: number) {
  const [queue, setQueue] = useState<Queue>();
  const [authz, setAuthz] = useState<QueueAuthz>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const invalid = !Number.isInteger(queueId) || queueId <= 0;

  const apply = useCallback((r: Result<{ queue: Queue; queue_authz: QueueAuthz }>) => {
    if (r.ok) {
      setQueue(r.data.queue);
      setAuthz(r.data.queue_authz);
      setError(undefined);
    } else {
      setError(r.message);
    }
    setLoading(false);
  }, []);

  const reload = useCallback(async () => {
    if (invalid) return;
    apply(await run(() => getQueue(queueId)));
  }, [apply, invalid, queueId]);

  useEffect(() => {
    if (invalid) return;
    let active = true;
    void run(() => getQueue(queueId)).then((r) => {
      if (active) apply(r);
    });
    return () => {
      active = false;
    };
  }, [apply, invalid, queueId]);

  if (invalid) {
    return { queue: undefined, authz: undefined, loading: false, error: 'Antrian tidak ditemukan', reload };
  }
  return { queue, authz, loading, error, reload };
}
