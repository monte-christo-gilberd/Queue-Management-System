import { useCallback, useEffect, useMemo, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { createCounter, deleteCounter, listCounters, renameCounter, type Counter } from '@/lib/counters';
import { describeCounterError } from '@/lib/queue-errors';
import { readStoredCounter, resolveDeviceCounter, switchNotice, writeStoredCounter } from '@/utils/device-counter';

/**
 * Sesi counter satu antrian untuk satu perangkat: daftar counter (dimuat ulang pada `counter_updated`),
 * counter yang dipakai perangkat ini (tersimpan per antrian), dan aksi buat, ganti nama, hapus, serta pindah.
 * Dashboard, detail antrian, dan papan memakai satu sumber ini supaya tidak berbeda pendapat.
 */
export function useCounterSession(queueId: number | null) {
  const [counters, setCounters] = useState<Counter[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [error, setError] = useState<string>();
  // Pilihan yang baru dibuat perangkat ini; dibaca sebelum localStorage supaya langsung berlaku.
  const [pick, setPick] = useState<{ queueId: number; id: number } | null>(null);
  const [notice, setNotice] = useState<string>();

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

  useQueueEvents(queueId, {
    onEvent: (event) => {
      if (event.type === 'counter_updated') void load();
    },
    onReconnect: () => void load(),
    onResync: () => void load(),
  });

  const storedId = useMemo(() => {
    if (queueId === null) return null;
    if (pick && pick.queueId === queueId) return pick.id;
    return readStoredCounter(queueId);
  }, [queueId, pick]);

  const counterId = queueId === null ? 0 : resolveDeviceCounter(storedId, counters);
  const current = counters.find((c) => c.id === counterId);
  // Pilihan tersimpan tidak ada lagi di daftar (dihapus dari perangkat lain): perangkat kembali ke default.
  const fellBack = loaded && storedId !== null && !counters.some((c) => c.id === storedId);

  const select = useCallback(
    (id: number) => {
      if (queueId === null || id === counterId) return;
      setNotice(switchNotice(current) ?? undefined);
      writeStoredCounter(queueId, id);
      setPick({ queueId, id });
    },
    [queueId, counterId, current],
  );

  /** Buat counter baru (nama otomatis) dan langsung pindah ke sana, seperti membuka tab baru. */
  const add = useCallback(async (): Promise<boolean> => {
    if (queueId === null) return false;
    try {
      const created = await createCounter(queueId);
      writeStoredCounter(queueId, created.id);
      setPick({ queueId, id: created.id });
      setNotice(undefined);
      await load();
      return true;
    } catch (e) {
      setError(describeCounterError(e));
      return false;
    }
  }, [queueId, load]);

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
