/**
 * use-queues: daftar antrian owner dan detail satu antrian.
 *
 * Dipakai oleh : src/app/(user)/dashboard.tsx, src/app/(user)/queues/index.tsx, src/app/(user)/queues/[queueId]/index.tsx, src/app/(user)/queues/[queueId]/live.tsx.
 * Bergantung   : lib/queues (listQueues, getQueue), lib/queue-errors (describeQueueError), types/api (Queue).
 * Alur         : muat awal lewat effect. `reload` dipakai setelah membuat atau menutup antrian.
 * Catatan      : Hasil request dibungkus Result (ok atau message) supaya galat tidak dilempar keluar effect.
 */

import { useCallback, useEffect, useState } from 'react';

import { describeQueueError } from '@/lib/queue-errors';
import { getQueue, listQueues } from '@/lib/queues';
import type { Queue } from '@/types/api';

/** Hasil request: `ok` true membawa `data`, false membawa pesan galat yang sudah ramah. */
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
 *
 * Tanpa argumen. Mengembalikan objek dengan field:
 * - `queues`: daftar antrian owner. Kosong sebelum dimuat.
 * - `loading`: true sampai percobaan pertama selesai.
 * - `error`: pesan galat terakhir. undefined bila tidak ada.
 * - `reload()`: memuat ulang daftar sekarang.
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

/**
 * Detail satu antrian. Id tidak valid langsung dianggap tidak ditemukan (tanpa request).
 *
 * Argumen: `queueId` adalah id antrian. Bila bukan bilangan bulat positif, hook langsung mengembalikan galat.
 *
 * Mengembalikan objek dengan field:
 * - `queue`: data antrian. undefined sebelum dimuat, atau bila id tidak valid.
 * - `loading`: true selama muat awal. false langsung untuk id tidak valid.
 * - `error`: pesan galat. Berisi "Antrian tidak ditemukan" untuk id tidak valid.
 * - `reload()`: memuat ulang detail. Tidak melakukan apa pun untuk id tidak valid.
 */
export function useQueueDetail(queueId: number) {
  const [queue, setQueue] = useState<Queue>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const invalid = !Number.isInteger(queueId) || queueId <= 0;

  const apply = useCallback((r: Result<{ queue: Queue }>) => {
    if (r.ok) {
      setQueue(r.data.queue);
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

  // Id tidak valid dikembalikan langsung tanpa menunggu effect, sehingga layar tidak menampilkan skeleton selamanya.
  if (invalid) {
    return { queue: undefined, loading: false, error: 'Antrian tidak ditemukan', reload };
  }
  return { queue, loading, error, reload };
}
