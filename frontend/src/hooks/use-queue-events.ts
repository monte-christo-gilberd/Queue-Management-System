/**
 * use-queue-events: langganan SSE satu antrian untuk pembaruan langsung di layar owner dan customer.
 *
 * Dipakai oleh : use-queue-board.ts, use-counter-session.ts, use-queue-spot.ts, use-ticket-queue.ts.
 * Bergantung   : expo-router (useIsFocused), expo/fetch, lib/sse (parser, backoff, resinkron), lib/api (API_BASE_URL), store/authStore (token).
 * Alur         : queueId dan token -> satu stream GET /queues/:id/event per pasangan (queueId, token), dibagi ke semua
 *                pemanggil yang berlangganan -> parser SSE -> handlers tiap pemanggil. Resinkron dipicu timer dan perubahan fokus atau app.
 * Catatan      : Memakai expo/fetch karena EventSource tidak bisa mengirim header Authorization. Handler disimpan di ref agar pemanggil tidak membuka ulang koneksi tiap render.
 *                Satu antrian bisa dipakai banyak hook sekaligus (papan dan sesi counter di dashboard owner). Tanpa berbagi, tiap hook membuka
 *                stream sendiri, dan setiap antrian memakai dua koneksi HTTP. Dengan registry ini hanya ada satu koneksi per antrian.
 */

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

/** Satu pemanggil yang berlangganan. `handlersRef` dibaca tiap event, jadi handler terbaru selalu yang dipakai. */
type Subscriber = { handlersRef: { current: Handlers } };

/** Satu stream bersama: dipakai oleh semua subscriber dengan (queueId, token) yang sama. */
type StreamEntry = {
  controller: AbortController;
  subs: Set<Subscriber>;
  health: { current: StreamHealth };
};

/** Registry stream aktif, kunci `${queueId}|${token}`. Entri dihapus saat subscriber terakhir pergi atau loop selesai. */
const streams = new Map<string, StreamEntry>();

function streamKey(queueId: number, token: string): string {
  return `${queueId}|${token}`;
}

/**
 * Mendaftarkan subscriber ke stream antrian ini. Stream dibuka saat subscriber pertama datang. Mengembalikan fungsi
 * untuk berhenti berlangganan: stream ditutup bila tidak ada lagi subscriber.
 */
function subscribe(queueId: number, token: string, sub: Subscriber): () => void {
  const key = streamKey(queueId, token);
  let entry = streams.get(key);
  if (!entry) {
    const created: StreamEntry = {
      controller: new AbortController(),
      subs: new Set(),
      health: { current: { live: false, since: Date.now() } },
    };
    entry = created;
    streams.set(key, created);
    void streamLoop(queueId, token, created).finally(() => {
      if (streams.get(key) === created) streams.delete(key);
    });
  }
  entry.subs.add(sub);

  return () => {
    const current = streams.get(key);
    if (!current) return;
    current.subs.delete(sub);
    if (current.subs.size === 0) {
      current.controller.abort();
      streams.delete(key);
    }
  };
}

/** Kesehatan stream untuk kunci ini; nilai awal bila belum ada stream. */
function healthOf(queueId: number, token: string): StreamHealth {
  return streams.get(streamKey(queueId, token))?.health.current ?? { live: false, since: Date.now() };
}

/**
 * Berlangganan SSE satu antrian lewat fetch-stream (EventSource tidak bisa mengirim header Authorization).
 *
 * - Satu stream per (antrian, token), dibagi ke semua pemanggil yang sedang aktif.
 * - Reconnect dengan backoff (1s → 30s). Reset setelah koneksi berhasil.
 * - 401 → sesi habis, logout. 403/404 → berhenti (bukan milik user / tidak ada).
 * - Resinkron (`onResync`) saat tab/app kembali aktif dan tiap 45 detik selama aktif; tiap 3 detik bila SSE
 *   tidak mengirim apa pun (mis. quick tunnel menahan stream). Timer ini per pemanggil.
 * - Dihentikan saat `queueId` berubah, komponen unmount, layar tidak fokus (layar di bawah stack tidak
 *   boleh menahan koneksi), atau antrian ditutup (`queue_closed` tidak pernah dibuka lagi).
 *
 * Argumen: `queueId` adalah id antrian yang diikuti, null berarti tidak berlangganan. `handlers` berisi
 * tiga callback (lihat tipe Handlers). Fungsi ini tidak mengembalikan nilai.
 */
export function useQueueEvents(queueId: number | null, handlers: Handlers) {
  const token = useAuthStore((s) => s.token);
  const focused = useIsFocused();

  // Handler disimpan di ref agar pemanggil boleh memberi fungsi baru tiap render tanpa membuka ulang koneksi.
  const handlersRef = useRef(handlers);
  useEffect(() => {
    handlersRef.current = handlers;
  });

  // Koneksi hanya dibuka bila ada antrian, token login, dan layar sedang fokus. Ganti antrian atau token melepas langganan lama.
  useEffect(() => {
    if (queueId === null || !token || !focused) return;
    return subscribe(queueId, token, { handlersRef });
  }, [queueId, token, focused]);

  // Timer resinkron terpisah dari stream: jalan walau stream diam, sehingga pemanggil tetap terjaga dari event yang terlewat.
  useEffect(() => {
    if (queueId === null || !token || !focused) return;
    let lastSyncAt = Date.now();
    let active = isForeground();

    // `becameActive` true saat app baru kembali ke depan: resinkron langsung, tanpa menunggu interval.
    function check(becameActive: boolean) {
      const now = Date.now();
      const { live, since } = healthOf(queueId as number, token as string);
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

/** Apakah app atau tab sedang tampil di depan. */
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

/**
 * Loop koneksi satu stream: membuka stream, membaca event sampai putus, lalu menunggu backoff (1 detik, naik ganda hingga 30 detik)
 * sebelum mencoba lagi. Backoff kembali ke awal setelah koneksi berhasil. Event dan pemulihan dikirim ke semua subscriber.
 * Loop berhenti saat signal dibatalkan, antrian ditutup, atau server menolak dengan 401/403/404.
 */
async function streamLoop(queueId: number, token: string, entry: StreamEntry) {
  const { signal } = entry.controller;
  let delay = BACKOFF_START_MS;
  let disconnected = false;
  // Antrian yang sudah ditutup tidak pernah dibuka lagi: berhenti, jangan reconnect.
  let closed = false;

  // Salinan daftar subscriber saat ini; subscriber yang berhenti di tengah event tidak ikut dipanggil lagi.
  const each = (fn: (handlers: Handlers) => void) => {
    for (const sub of Array.from(entry.subs)) {
      if (entry.subs.has(sub)) fn(sub.handlersRef.current);
    }
  };

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

      if (disconnected) each((h) => h.onReconnect());
      disconnected = false;
      delay = BACKOFF_START_MS;

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      const parser = createSseParser((message) => {
        const event = parseQueueEvent(message.data);
        if (!event) return;
        each((h) => h.onEvent(event));
        if (event.type === 'queue_closed') closed = true;
      });

      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        entry.health.current.live = true;
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
    entry.health.current = { live: false, since: Date.now() };
    disconnected = true;
    await sleep(delay, signal);
    delay = nextBackoff(delay);
  }
}

/** Tidur selama `ms`, tetapi langsung bangun saat signal dibatalkan agar unmount tidak menunggu backoff penuh. */
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
