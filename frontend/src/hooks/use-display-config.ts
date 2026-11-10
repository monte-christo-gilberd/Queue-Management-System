/**
 * use-display-config: konfigurasi layar publik sebuah antrian untuk owner (aktifkan, atur jumlah, putar token, nonaktifkan).
 *
 * Dipakai oleh : src/app/(owner)/queues/[queueId]/index.tsx.
 * Bergantung   : lib/display (getDisplayConfig, saveDisplayConfig, rotateDisplay, disableDisplay, describeDisplayError).
 * Alur         : queueId -> getDisplayConfig saat muat awal. Setiap aksi -> lib/display -> config baru menggantikan state.
 * Catatan      : Satu aksi berjalan pada satu waktu lewat `busy`. Tidak ada cache lintas antrian.
 */

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
 *
 * Argumen: `queueId` adalah id antrian. Wajib angka positif.
 *
 * Mengembalikan objek dengan field:
 * - `config`: konfigurasi layar saat ini. null bila layar belum diaktifkan.
 * - `loading`: true selama muat awal.
 * - `busy`: true selama satu aksi berjalan. Tombol dinonaktifkan selama itu.
 * - `error`: pesan galat muat atau aksi. undefined bila tidak ada.
 * - `enable()`: aktifkan layar publik untuk antrian ini.
 * - `setCounts({ upcoming_count?, skipped_count? })`: ubah jumlah nomor yang tampil di layar.
 * - `rotate()`: ganti token layar. Tautan lama tidak berlaku lagi.
 * - `disable()`: nonaktifkan layar. `config` menjadi null.
 */
export function useDisplayConfig(queueId: number) {
  const [config, setConfig] = useState<DisplayConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  // Muat awal dengan then(), supaya setState tidak sinkron di dalam effect. Galat ditampilkan, tidak dilempar.
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
  // Backend tidak mengembalikan konfigurasi saat dinonaktifkan, jadi state diset null secara eksplisit.
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
