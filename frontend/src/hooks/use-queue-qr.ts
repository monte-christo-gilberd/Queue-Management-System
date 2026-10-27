import { useCallback, useEffect, useRef, useState } from 'react';
import { AppState } from 'react-native';

import { getQueueQR, replaceQueueQR, setQueueQRType } from '@/lib/qr';
import type { QueueQR } from '@/types/api';

/** Refresh otomatis token dinamis ketika sisa masa berlaku < 20 detik. */
const DYNAMIC_REFRESH_LEAD_SECONDS = 20;

/**
 * State dan aksi QR antrian (owner): muat ulang, toggle statis/dinamis, ganti kode.
 *
 * Token dinamis membawa `expires_at`; hitung mundur berjalan saat app aktif dan QR
 * dimuat ulang otomatis sekali ketika sisa waktu di bawah 20 detik.
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
      setError(e instanceof Error ? e.message : 'Gagal memuat QR');
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
        setError(e instanceof Error ? e.message : 'Gagal memuat QR');
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

  // Countdown token dinamis.
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

  // Token dinamis: reload otomatis sekali saat sisa masa di bawah lead.
  const autoRefreshedFor = useRef<number | null>(null);
  useEffect(() => {
    if (!appActive || expiresAtMs == null || secondsLeft == null) return;
    if (secondsLeft > DYNAMIC_REFRESH_LEAD_SECONDS) return;
    if (autoRefreshedFor.current === expiresAtMs) return;
    autoRefreshedFor.current = expiresAtMs;
    const timer = setTimeout(() => void reload(), 1000);
    return () => clearTimeout(timer);
  }, [appActive, expiresAtMs, secondsLeft, reload]);

  const toggle = useCallback(
    async (type: 'static' | 'dynamic') => {
      if (invalid) return;
      setToggling(true);
      setError(undefined);
      try {
        setQr(await setQueueQRType(queueId, type));
        autoRefreshedFor.current = null;
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Gagal mengubah mode QR');
      } finally {
        setToggling(false);
      }
    },
    [invalid, queueId],
  );

  const replace = useCallback(async () => {
    if (invalid) return;
    setReplacing(true);
    setError(undefined);
    try {
      setQr(await replaceQueueQR(queueId));
      autoRefreshedFor.current = null;
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Gagal mengganti QR');
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
