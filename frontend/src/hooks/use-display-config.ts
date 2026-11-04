import { useCallback, useEffect, useState } from 'react';

import {
  describeDisplayError,
  disableDisplay,
  getDisplayConfig,
  rotateDisplay,
  saveDisplayConfig,
  type DisplayConfig,
} from '@/lib/display';

/**
 * Muat dan ubah konfigurasi layar publik sebuah antrian (owner). `config` null = layar belum diaktifkan.
 * Memuat sekali per `queueId`: pemanggil memberi `key` per antrian (halaman detail sudah begitu), jadi berpindah
 * antrian memasang ulang hook ini dengan keadaan awal yang bersih.
 */
export function useDisplayConfig(queueId: number) {
  const [config, setConfig] = useState<DisplayConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    let active = true;
    void getDisplayConfig(queueId).then(
      (c) => {
        if (!active) return;
        setConfig(c);
        setLoading(false);
      },
      (e: unknown) => {
        if (!active) return;
        setError(describeDisplayError(e));
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
  }, [queueId]);

  // Satu aksi pada satu waktu: tombol dinonaktifkan selama `busy`, jadi jawaban tidak saling mendahului.
  const run = useCallback(async (action: () => Promise<DisplayConfig | null>) => {
    setBusy(true);
    setError(undefined);
    try {
      setConfig(await action());
    } catch (e) {
      setError(describeDisplayError(e));
    } finally {
      setBusy(false);
    }
  }, []);

  const enable = useCallback(() => run(() => saveDisplayConfig(queueId)), [run, queueId]);
  const setCounts = useCallback(
    (counts: { upcoming_count?: number; skipped_count?: number }) => run(() => saveDisplayConfig(queueId, counts)),
    [run, queueId],
  );
  const rotate = useCallback(() => run(() => rotateDisplay(queueId)), [run, queueId]);
  const disable = useCallback(
    () =>
      run(async () => {
        await disableDisplay(queueId);
        return null;
      }),
    [run, queueId],
  );

  return { config, loading, busy, error, enable, setCounts, rotate, disable };
}
