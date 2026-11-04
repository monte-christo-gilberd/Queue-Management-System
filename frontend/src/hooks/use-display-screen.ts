import { fetch as expoFetch } from 'expo/fetch';
import { useCallback, useEffect, useRef, useState } from 'react';
import { Platform } from 'react-native';

import { ApiError, API_BASE_URL } from '@/lib/api';
import { getPublicDisplay, type DisplaySnapshot } from '@/lib/display';
import { BACKOFF_START_MS, createSseParser, nextBackoff, RESYNC_INTERVAL_MS, FALLBACK_POLL_MS } from '@/lib/sse';
import { changedCounterIds, isConnected, SSE_FRESH_MS } from '@/utils/display';

export type DisplayStatus = 'loading' | 'ready' | 'invalid' | 'error';

/** Lama sorotan kartu yang nomornya baru berganti. */
export const HIGHLIGHT_MS = 1500;
// Beberapa sinyal beruntun (mis. Skip memancarkan dua) digabung menjadi satu pengambilan.
const EVENT_DEBOUNCE_MS = 150;

/**
 * Data layar publik: snapshot dari GET /display/:token, dibangunkan sinyal SSE tanpa login (`display_changed`),
 * dengan resinkron cadangan (45 detik bila stream berdetak, 3 detik bila tidak). Tidak memakai token login.
 *
 * - `connected` false bila stream dan pengambilan sama-sama basi: layar menampilkan "menyambung ulang".
 * - `status` `invalid` = tautan tidak berlaku (404 dari snapshot atau stream); loop berhenti.
 * - Mencegah layar tidur di web (Wake Lock) bila peramban mendukung.
 * - Satu `token` per pemasangan: pemanggil memberi `key={token}` supaya berpindah token memasang ulang hook ini.
 */
export function useDisplayScreen(token: string) {
  const [snapshot, setSnapshot] = useState<DisplaySnapshot | null>(null);
  const [status, setStatus] = useState<DisplayStatus>('loading');
  const [connected, setConnected] = useState(true);
  const [highlighted, setHighlighted] = useState<number[]>([]);

  const snapshotRef = useRef<DisplaySnapshot | null>(null);
  const lastByteAt = useRef<number | null>(null);
  const lastFetchOkAt = useRef<number | null>(null);
  const invalidRef = useRef(false);
  const highlightTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const refresh = useCallback(async () => {
    if (invalidRef.current) return;
    try {
      const next = await getPublicDisplay(token);
      const changed = changedCounterIds(snapshotRef.current, next);
      snapshotRef.current = next;
      lastFetchOkAt.current = Date.now();
      setSnapshot(next);
      setStatus('ready');
      if (changed.length > 0) {
        setHighlighted(changed);
        if (highlightTimer.current) clearTimeout(highlightTimer.current);
        highlightTimer.current = setTimeout(() => setHighlighted([]), HIGHLIGHT_MS);
      }
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        invalidRef.current = true;
        setStatus('invalid');
        return;
      }
      // Jaringan atau server bermasalah: angka terakhir dipertahankan, `connected` yang menandai basi.
      if (snapshotRef.current === null) setStatus('error');
    }
  }, [token]);

  useEffect(() => {
    invalidRef.current = false;
    snapshotRef.current = null;
    lastByteAt.current = null;
    lastFetchOkAt.current = null;
    const controller = new AbortController();
    let debounce: ReturnType<typeof setTimeout> | null = null;

    function scheduleRefresh() {
      if (debounce) return;
      debounce = setTimeout(() => {
        debounce = null;
        void refresh();
      }, EVENT_DEBOUNCE_MS);
    }

    // Dijadwalkan (bukan dipanggil langsung) agar setState di dalam refresh tidak berjalan sinkron di badan effect.
    const first = setTimeout(() => void refresh(), 0);
    void streamLoop(token, controller.signal, {
      onByte: () => {
        lastByteAt.current = Date.now();
      },
      // Stream putus: detaknya tidak lagi dihitung, jadi status sambungan hanya bergantung pada polling (10 detik).
      onDisconnect: () => {
        lastByteAt.current = null;
      },
      onSignal: scheduleRefresh,
      onReconnect: () => void refresh(),
      onInvalid: () => {
        invalidRef.current = true;
        setStatus('invalid');
      },
    });

    // Tiap detik: perbarui status sambungan, dan resinkron bila sudah waktunya.
    let lastSync = Date.now();
    const timer = setInterval(() => {
      const now = Date.now();
      setConnected(isConnected(now, lastByteAt.current, lastFetchOkAt.current));
      const sseAlive = lastByteAt.current !== null && now - lastByteAt.current < SSE_FRESH_MS;
      if (now - lastSync >= (sseAlive ? RESYNC_INTERVAL_MS : FALLBACK_POLL_MS)) {
        lastSync = now;
        void refresh();
      }
    }, 1000);

    return () => {
      controller.abort();
      clearTimeout(first);
      clearInterval(timer);
      if (debounce) clearTimeout(debounce);
      if (highlightTimer.current) clearTimeout(highlightTimer.current);
    };
  }, [token, refresh]);

  useWakeLock(status === 'ready');

  return { snapshot, status, connected, highlighted };
}

type StreamHandlers = {
  onByte: () => void;
  onDisconnect: () => void;
  onSignal: () => void;
  onReconnect: () => void;
  onInvalid: () => void;
};

async function streamLoop(token: string, signal: AbortSignal, h: StreamHandlers) {
  let delay = BACKOFF_START_MS;
  let disconnected = false;

  while (!signal.aborted) {
    try {
      const res = await expoFetch(`${API_BASE_URL}/api/v1/display/${encodeURIComponent(token)}/event`, {
        headers: { Accept: 'text/event-stream' },
        signal,
      });
      if (res.status === 404) {
        h.onInvalid();
        return;
      }
      if (!res.ok || !res.body) throw new Error(`SSE status ${res.status}`);

      if (disconnected) h.onReconnect();
      disconnected = false;
      delay = BACKOFF_START_MS;

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      // Isi sinyal tidak dipakai (tanpa data); kedatangannya berarti "ambil ulang snapshot".
      const parser = createSseParser(() => h.onSignal());
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        h.onByte();
        parser.push(decoder.decode(value, { stream: true }));
      }
    } catch {
      // jaringan putus, server restart, atau dibatalkan (dicek di bawah)
    }
    if (signal.aborted) return;
    h.onDisconnect();
    disconnected = true;
    await sleep(delay, signal);
    delay = nextBackoff(delay);
  }
}

function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const timer = setTimeout(resolve, ms);
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(timer);
        resolve();
      },
      { once: true },
    );
  });
}

/** Web: tahan layar agar tidak tidur selama layar publik tampil. Diam bila tidak didukung atau ditolak. */
function useWakeLock(active: boolean) {
  useEffect(() => {
    if (!active || Platform.OS !== 'web' || typeof navigator === 'undefined') return;
    const wakeLock = (navigator as Navigator & { wakeLock?: { request: (t: 'screen') => Promise<WakeLockSentinel> } })
      .wakeLock;
    if (!wakeLock) return;
    let sentinel: WakeLockSentinel | null = null;
    let cancelled = false;

    async function acquire() {
      try {
        const s = await wakeLock!.request('screen');
        if (cancelled) void s.release();
        else sentinel = s;
      } catch {
        // ditolak (mis. baterai hemat atau tab di latar belakang)
      }
    }
    // Wake lock dilepas peramban saat tab tersembunyi; minta lagi saat tampil kembali.
    const onVisible = () => {
      if (document.visibilityState === 'visible') void acquire();
    };
    void acquire();
    document.addEventListener('visibilitychange', onVisible);
    return () => {
      cancelled = true;
      document.removeEventListener('visibilitychange', onVisible);
      void sentinel?.release();
    };
  }, [active]);
}
