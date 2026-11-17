/**
 * use-queue-qr: QR antrian untuk owner (muat, ganti mode statis atau dinamis, ganti kode, hitung mundur token dinamis).
 *
 * Dipakai oleh : src/app/(user)/queues/[queueId]/index.tsx.
 * Bergantung   : lib/qr (getQueueQR, replaceQueueQR, setQueueQRType), lib/qr-errors (describeQrError), types/api (QueueQR), react-native (AppState).
 * Alur         : queueId -> getQueueQR -> qr. Hitung mundur dari expires_at. Saat kedaluwarsa, reload dicoba tiap 3 detik sampai token baru datang.
 * Catatan      : Hitung mundur dan timer hanya berjalan saat app aktif. Ref `wasActive` mendeteksi kembali dari background supaya reload tidak dipicu tiap perubahan state.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { AppState } from 'react-native';

import { describeQrError } from '@/lib/qr-errors';
import { getQueueQR, replaceQueueQR, setQueueQRType } from '@/lib/qr';
import type { QueueQR } from '@/types/api';

/** Jeda antar percobaan ulang saat token dinamis sudah kedaluwarsa dan belum berganti. */
const EXPIRED_RETRY_MS = 3000;

/**
 * State dan aksi QR antrian (owner): muat ulang, toggle statis/dinamis, ganti kode.
 *
 * Token dinamis membawa `expires_at`; hitung mundur berjalan saat app aktif dan QR
 * diganti otomatis setelah kedaluwarsa (dicoba ulang tiap 3 detik sampai token baru masuk).
 *
 * Argumen: `queueId` adalah id antrian. Bila bukan bilangan bulat positif, hook tidak membuat request dan `loading` langsung false.
 *
 * Mengembalikan objek dengan field:
 * - `qr`: data QR terakhir. null sebelum dimuat.
 * - `loading`: true selama muat awal (skeleton). Refresh diam-diam tidak mengubahnya.
 * - `error`: pesan galat terakhir. undefined bila tidak ada.
 * - `toggling`: true selama ganti mode statis atau dinamis.
 * - `replacing`: true selama ganti kode.
 * - `countdown`: sisa detik token dinamis. null bila QR statis atau belum ada expires_at.
 * - `reload()`: muat ulang diam-diam, QR lama tetap tampil selama itu.
 * - `refresh()`: muat ulang dengan skeleton, dengan memaksa effect muat awal berjalan lagi.
 * - `toggle(type)`: ganti mode ke `static` atau `dynamic`.
 * - `replace()`: ganti kode QR dengan kode baru.
 */
export function useQueueQR(queueId: number) {
  const invalid = !Number.isInteger(queueId) || queueId <= 0;

  const [qr, setQr] = useState<QueueQR | null>(null);
  const [loading, setLoading] = useState(!invalid);
  const [error, setError] = useState<string>();
  const [toggling, setToggling] = useState(false);
  const [replacing, setReplacing] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [appActive, setAppActive] = useState(AppState.currentState === 'active');

  // Refresh diam-diam (auto-refresh/countdown): tanpa skeleton, QR lama tetap tampil.
  const reload = useCallback(async () => {
    if (invalid) return;
    setError(undefined);
    try {
      setQr(await getQueueQR(queueId));
    } catch (e) {
      setError(describeQrError(e));
    }
  }, [invalid, queueId]);

  // Muat awal / refresh. `.then()` agar setState tidak sinkron di dalam effect.
  useEffect(() => {
    if (invalid) return;
    let active = true;
    void getQueueQR(queueId).then(
      (data) => {
        if (!active) return;
        setQr(data);
        setError(undefined);
        setLoading(false);
      },
      (e: unknown) => {
        if (!active) return;
        setError(describeQrError(e));
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
  }, [invalid, queueId, reloadKey]);

  const refresh = useCallback(() => {
    setLoading(true);
    setReloadKey((k) => k + 1);
  }, []);

  // Countdown token dinamis. expires_at dari server dalam detik, diubah ke milidetik agar selisih dengan Date.now() cocok.
  const expiresAtMs = qr?.type === 'dynamic' && qr.expires_at != null ? qr.expires_at * 1000 : null;
  const [secondsLeft, setSecondsLeft] = useState<number | null>(null);

  useEffect(() => {
    if (expiresAtMs == null || !appActive) return;
    const tick = () => setSecondsLeft(Math.max(0, Math.ceil((expiresAtMs - Date.now()) / 1000)));
    const initial = setTimeout(tick, 0);
    const timer = setInterval(tick, 1000);
    return () => {
      clearTimeout(initial);
      clearInterval(timer);
    };
  }, [appActive, expiresAtMs]);

  // Saat kembali dari background, sisa waktu bisa saja sudah lewat: muat ulang.
  // Ref `wasActive` menyimpan status sebelumnya, supaya reload hanya jalan saat transisi dari background ke aktif.
  const wasActive = useRef(AppState.currentState === 'active');
  useEffect(() => {
    const subscription = AppState.addEventListener('change', (state) => {
      const active = state === 'active';
      setAppActive(active);
      if (active && !wasActive.current) void reload();
      wasActive.current = active;
    });
    return () => subscription.remove();
  }, [reload]);

  // Token diganti setelah kedaluwarsa: minta token baru, lalu coba lagi tiap beberapa detik
  // sampai backend mengeluarkan token baru (expiresAtMs berubah).
  const expired = secondsLeft === 0;
  useEffect(() => {
    if (!appActive || expiresAtMs == null || !expired) return;
    const first = setTimeout(() => void reload(), 0);
    const timer = setInterval(() => void reload(), EXPIRED_RETRY_MS);
    return () => {
      clearTimeout(first);
      clearInterval(timer);
    };
  }, [appActive, expiresAtMs, expired, reload]);

  // Ganti mode QR. Hasil server langsung menggantikan `qr`, dan `toggling` mengunci tombol selama request.
  const toggle = useCallback(
    async (type: 'static' | 'dynamic') => {
      if (invalid) return;
      setToggling(true);
      setError(undefined);
      try {
        setQr(await setQueueQRType(queueId, type));
          } catch (e) {
        setError(describeQrError(e));
      } finally {
        setToggling(false);
      }
    },
    [invalid, queueId],
  );

  // Ganti kode QR. Hasil server langsung menggantikan `qr`, dan `replacing` mengunci tombol selama request.
  const replace = useCallback(async () => {
    if (invalid) return;
    setReplacing(true);
    setError(undefined);
    try {
      setQr(await replaceQueueQR(queueId));
    } catch (e) {
      setError(describeQrError(e));
    } finally {
      setReplacing(false);
    }
  }, [invalid, queueId]);

  return {
    qr,
    loading,
    error,
    toggling,
    replacing,
    countdown: expiresAtMs == null ? null : secondsLeft,
    reload,
    refresh,
    toggle,
    replace,
  };
}
