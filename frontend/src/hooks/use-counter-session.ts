/**
 * use-counter-session: sesi counter satu antrian untuk satu perangkat owner (daftar, pilihan aktif, buat, ganti nama, hapus).
 *
 * Dipakai oleh : src/app/(owner)/dashboard.tsx, src/app/(owner)/queues/[queueId]/index.tsx, src/app/(owner)/queues/[queueId]/live.tsx, src/components/CounterBar.tsx.
 * Bergantung   : hooks/use-queue-events (SSE), lib/counters, lib/queue-errors (describeCounterError), utils/device-counter (baca dan tulis pilihan perangkat, resolveDeviceCounter, switchNotice).
 * Alur         : queueId -> listCounters -> counters. Pilihan perangkat dibaca dari localStorage kunci `qms-device-counter:<queueId>`. Event counter_updated memuat ulang daftar.
 * Catatan      : Pilihan yang baru dibuat disimpan di state dulu agar langsung berlaku sebelum localStorage terbaca ulang. Pilihan yang sudah tidak ada di daftar jatuh kembali ke counter default.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { createCounter, deleteCounter, listCounters, renameCounter, type Counter } from '@/lib/counters';
import { describeCounterError } from '@/lib/queue-errors';
import { readStoredCounter, resolveDeviceCounter, switchNotice, writeStoredCounter } from '@/utils/device-counter';

/**
 * Sesi counter satu antrian untuk satu perangkat: daftar counter (dimuat ulang pada `counter_updated`),
 * counter yang dipakai perangkat ini (tersimpan per antrian), dan aksi buat, ganti nama, hapus, serta pindah.
 * Dashboard, detail antrian, dan papan memakai satu sumber ini supaya tidak berbeda pendapat.
 *
 * Argumen: `queueId` adalah id antrian yang dikelola. null berarti belum dipilih, sehingga tidak ada request.
 *
 * Mengembalikan objek dengan field:
 * - `counters`: daftar counter antrian ini.
 * - `loaded`: true setelah percobaan muat pertama selesai, berhasil atau tidak.
 * - `error`: pesan galat muat daftar. undefined bila berhasil.
 * - `counterId`: counter yang dipakai perangkat ini. 0 bila daftar belum dimuat.
 * - `current`: objek counter yang dipakai, undefined bila tidak ada.
 * - `fellBack`: true bila pilihan tersimpan sudah tidak ada dan perangkat kembali ke default.
 * - `notice`: peringatan saat pindah counter, undefined bila tidak ada.
 * - `select(id)`: pindah ke counter `id` dan menyimpannya untuk perangkat ini.
 * - `add(name?)`: buat counter baru dan langsung pindah ke sana. Mengembalikan pesan galat atau null.
 * - `rename(id, name)`: ganti nama counter. Mengembalikan pesan galat atau null.
 * - `remove(id)`: hapus counter. Mengembalikan pesan galat atau null.
 */
export function useCounterSession(queueId: number | null) {
  const [counters, setCounters] = useState<Counter[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string>();
  // Pilihan yang baru dibuat perangkat ini; dibaca sebelum localStorage supaya langsung berlaku.
  const [pick, setPick] = useState<{ queueId: number; id: number } | null>(null);
  const [notice, setNotice] = useState<string>();

  // Muat ulang daftar. Galat disimpan di `error`, dan `loaded` selalu menjadi true setelah selesai.
  const load = useCallback(async () => {
    if (queueId === null) return;
    try {
      setCounters(await listCounters(queueId));
      setError(undefined);
    } catch (e) {
      setError(describeCounterError(e));
    } finally {
      setLoaded(true);
    }
  }, [queueId]);

  // Muat awal dengan `.then()` agar setState tidak terjadi sinkron di dalam effect.
  useEffect(() => {
    if (queueId === null) return;
    let active = true;
    void listCounters(queueId).then(
      (data) => {
        if (!active) return;
        setCounters(data);
        setError(undefined);
        setLoaded(true);
      },
      (e: unknown) => {
        if (!active) return;
        setError(describeCounterError(e));
        setLoaded(true);
      },
    );
    return () => {
      active = false;
    };
  }, [queueId]);

  // Counter berubah di perangkat lain (buat, ganti nama, atau hapus) memicu muat ulang. Pemulihan koneksi dan resinkron juga memuat ulang, karena event bisa terlewat.
  useQueueEvents(queueId, {
    onEvent: (event) => {
      if (event.type === 'counter_updated') void load();
    },
    onReconnect: () => void load(),
    onResync: () => void load(),
  });

  // Pilihan dari state (`pick`) menang atas localStorage, tetapi hanya untuk antrian yang sama.
  const storedId = useMemo(() => {
    if (queueId === null) return null;
    if (pick && pick.queueId === queueId) return pick.id;
    return readStoredCounter(queueId);
  }, [queueId, pick]);

  const counterId = queueId === null ? 0 : resolveDeviceCounter(storedId, counters);
  const current = counters.find((c) => c.id === counterId);
  // Pilihan tersimpan tidak ada lagi di daftar (dihapus dari perangkat lain): perangkat kembali ke default.
  const fellBack = loaded && storedId !== null && !counters.some((c) => c.id === storedId);

  // Memilih counter yang sama tidak melakukan apa-apa. Counter lama yang masih memegang nomor diberi peringatan, tetapi tetap bisa dikembalikan kapan saja.
  const select = useCallback(
    (id: number) => {
      if (queueId === null || id === counterId) return;
      setNotice(switchNotice(current) ?? undefined);
      writeStoredCounter(queueId, id);
      setPick({ queueId, id });
    },
    [queueId, counterId, current],
  );

  /**
   * Buat counter baru dengan nama (kosong = nama otomatis dari server) lalu langsung pindah ke sana,
   * seperti membuka tab baru. Mengembalikan pesan galat, atau null bila berhasil.
   */
  const add = useCallback(
    async (name?: string): Promise<string | null> => {
      if (queueId === null) return 'Antrian belum dimuat';
      try {
        // Counter baru langsung menjadi pilihan perangkat ini, lalu daftar dimuat ulang agar nomor dan nama terbaru tampil.
        const created = await createCounter(queueId, name);
        writeStoredCounter(queueId, created.id);
        setPick({ queueId, id: created.id });
        setNotice(undefined);
        await load();
        return null;
      } catch (e) {
        return describeCounterError(e);
      }
    },
    [queueId, load],
  );

  /** Ganti nama. Mengembalikan pesan galat, atau null bila berhasil. */
  const rename = useCallback(
    async (counterIdToRename: number, name: string): Promise<string | null> => {
      if (queueId === null) return 'Antrian belum dimuat';
      try {
        await renameCounter(queueId, counterIdToRename, name);
        await load();
        return null;
      } catch (e) {
        return describeCounterError(e);
      }
    },
    [queueId, load],
  );

  /** Hapus counter. Mengembalikan pesan galat, atau null bila berhasil. */
  const remove = useCallback(
    async (counterIdToRemove: number): Promise<string | null> => {
      if (queueId === null) return 'Antrian belum dimuat';
      try {
        await deleteCounter(queueId, counterIdToRemove);
        await load();
        return null;
      } catch (e) {
        return describeCounterError(e);
      }
    },
    [queueId, load],
  );

  return {
    counters,
    loaded,
    error,
    counterId,
    current,
    fellBack,
    notice,
    select,
    add,
    rename,
    remove,
  };
}
