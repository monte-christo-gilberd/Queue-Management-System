import { fetch as expoFetch } from 'expo/fetch';
import { useEffect, useRef } from 'react';

import { API_BASE_URL } from '@/lib/api';
import { BACKOFF_START_MS, createSseParser, nextBackoff, parseQueueEvent, type QueueEvent } from '@/lib/sse';
import { useAuthStore } from '@/store/authStore';

type Handlers = {
  /** Event yang sudah tervalidasi. */
  onEvent: (event: QueueEvent) => void;
  /** Dipanggil setelah koneksi pulih dari putus: event yang terlewat harus diambil ulang oleh pemanggil. */
  onReconnect: () => void;
};

/**
 * Berlangganan SSE satu antrian lewat fetch-stream (EventSource tidak bisa mengirim header Authorization).
 *
 * - Reconnect dengan backoff (1s → 30s). Reset setelah koneksi berhasil.
 * - 401 → sesi habis, logout. 403/404 → berhenti (bukan milik user / tidak ada).
 * - Dihentikan saat `queueId` berubah atau komponen unmount.
 */
export function useQueueEvents(queueId: number | null, handlers: Handlers) {
  const token = useAuthStore((s) => s.token);

  // Handler disimpan di ref agar pemanggil boleh memberi fungsi baru tiap render tanpa membuka ulang koneksi.
  const handlersRef = useRef(handlers);
  useEffect(() => {
    handlersRef.current = handlers;
  });

  useEffect(() => {
    if (queueId === null || !token) return;
    const controller = new AbortController();
    void streamLoop(queueId, token, controller.signal, handlersRef);
    return () => controller.abort();
  }, [queueId, token]);
}

async function streamLoop(
  queueId: number,
  token: string,
  signal: AbortSignal,
  handlersRef: { current: Handlers },
) {
  let delay = BACKOFF_START_MS;
  let disconnected = false;

  while (!signal.aborted) {
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
        if (event) handlersRef.current.onEvent(event);
      });

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        parser.push(decoder.decode(value, { stream: true }));
      }
    } catch {
      // jaringan putus, server restart, atau request dibatalkan (dicek di bawah)
    }

    if (signal.aborted) return;
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
