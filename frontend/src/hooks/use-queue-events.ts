import { useIsFocused } from 'expo-router';
import { fetch as expoFetch } from 'expo/fetch';
import { useEffect, useRef } from 'react';
import { AppState, Platform } from 'react-native';

import { API_BASE_URL } from '@/lib/api';
import {
  BACKOFF_START_MS,
  createSseParser,
  nextBackoff,
  parseQueueEvent,
  resyncInterval,
  shouldResync,
  type QueueEvent,
} from '@/lib/sse';
import { useAuthStore } from '@/store/authStore';

/** Kesehatan stream: `live` jadi true setelah byte pertama diterima; `since` = awal percobaan/putus terakhir. */
type StreamHealth = { live: boolean; since: number };

type Handlers = {
  /** Event yang sudah tervalidasi. */
  onEvent: (event: QueueEvent) => void;
  /** Dipanggil setelah koneksi pulih dari putus: event yang terlewat harus diambil ulang oleh pemanggil. */
  onReconnect: () => void;
  /** Jaring pengaman: event SSE bisa terbuang. Dipanggil saat tab aktif kembali dan tiap 45 detik selama aktif. */
  onResync: () => void;
};

/**
 * Berlangganan SSE satu antrian lewat fetch-stream (EventSource tidak bisa mengirim header Authorization).
 *
 * - Reconnect dengan backoff (1s → 30s). Reset setelah koneksi berhasil.
 * - 401 → sesi habis, logout. 403/404 → berhenti (bukan milik user / tidak ada).
 * - Resinkron (`onResync`) saat tab/app kembali aktif dan tiap 45 detik selama aktif; tiap 3 detik bila SSE
 *   tidak mengirim apa pun (mis. quick tunnel menahan stream).
 * - Dihentikan saat `queueId` berubah, komponen unmount, layar tidak fokus (layar di bawah stack tidak
 *   boleh menahan koneksi), atau antrian ditutup (`queue_closed` tidak pernah dibuka lagi).
 */
export function useQueueEvents(queueId: number | null, handlers: Handlers) {
  const token = useAuthStore((s) => s.token);
  const focused = useIsFocused();

  // Handler disimpan di ref agar pemanggil boleh memberi fungsi baru tiap render tanpa membuka ulang koneksi.
  const handlersRef = useRef(handlers);
  useEffect(() => {
    handlersRef.current = handlers;
  });

  const healthRef = useRef<StreamHealth>({ live: false, since: Date.now() });

  useEffect(() => {
    if (queueId === null || !token || !focused) return;
    const controller = new AbortController();
    healthRef.current = { live: false, since: Date.now() };
    void streamLoop(queueId, token, controller.signal, handlersRef, healthRef);
    return () => controller.abort();
  }, [queueId, token, focused]);

  useEffect(() => {
    if (queueId === null || !token || !focused) return;
    let lastSyncAt = Date.now();
    let active = isForeground();

    function check(becameActive: boolean) {
      const now = Date.now();
      const { live, since } = healthRef.current;
      if (!shouldResync(active, becameActive, lastSyncAt, now, resyncInterval(live, since, now))) return;
      lastSyncAt = now;
      handlersRef.current.onResync();
    }

    const timer = setInterval(() => check(false), 1000);
    const stop = onForegroundChange((nowActive) => {
      const becameActive = nowActive && !active;
      active = nowActive;
      check(becameActive);
    });
    return () => {
      clearInterval(timer);
      stop();
    };
  }, [queueId, token, focused]);
}

function isForeground(): boolean {
  if (Platform.OS === 'web' && typeof document !== 'undefined') return document.visibilityState === 'visible';
  return AppState.currentState === 'active';
}

/** Web: `visibilitychange`. Native: `AppState`. Mengembalikan fungsi untuk berhenti. */
function onForegroundChange(listener: (active: boolean) => void): () => void {
  if (Platform.OS === 'web' && typeof document !== 'undefined') {
    const handler = () => listener(document.visibilityState === 'visible');
    document.addEventListener('visibilitychange', handler);
    return () => document.removeEventListener('visibilitychange', handler);
  }
  const sub = AppState.addEventListener('change', (state) => listener(state === 'active'));
  return () => sub.remove();
}

async function streamLoop(
  queueId: number,
  token: string,
  signal: AbortSignal,
  handlersRef: { current: Handlers },
  healthRef: { current: StreamHealth },
) {
  let delay = BACKOFF_START_MS;
  let disconnected = false;
  // Antrian yang sudah ditutup tidak pernah dibuka lagi: berhenti, jangan reconnect.
  let closed = false;

  while (!signal.aborted && !closed) {
    try {
      const res = await expoFetch(`${API_BASE_URL}/api/v1/queues/${queueId}/event`, {
        headers: { Authorization: `Bearer ${token}`, Accept: 'text/event-stream' },
        signal,
      });

      if (res.status === 401) {
        useAuthStore.getState().logout();
        return;
      }
      if (res.status === 403 || res.status === 404) return;
      if (!res.ok || !res.body) throw new Error(`SSE status ${res.status}`);

      if (disconnected) handlersRef.current.onReconnect();
      disconnected = false;
      delay = BACKOFF_START_MS;

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      const parser = createSseParser((message) => {
        const event = parseQueueEvent(message.data);
        if (!event) return;
        handlersRef.current.onEvent(event);
        if (event.type === 'queue_closed') closed = true;
      });

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        healthRef.current.live = true;
        parser.push(decoder.decode(value, { stream: true }));
        // Server tidak menutup stream sendiri setelah queue_closed: lepas koneksinya di sini.
        if (closed) {
          void reader.cancel();
          break;
        }
      }
    } catch {
      // jaringan putus, server restart, atau request dibatalkan (dicek di bawah)
    }

    if (signal.aborted || closed) return;
    healthRef.current = { live: false, since: Date.now() };
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
