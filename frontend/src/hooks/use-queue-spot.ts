/**
 * use-queue-spot: status satu antrian untuk kartu "nomor saya" di dashboard, diperbarui lewat SSE.
 *
 * Dipakai oleh : src/app/(user)/dashboard.tsx (kartu nomor yang dipegang akun ini).
 * Bergantung   : hooks/use-queue-events (SSE), lib/tickets (getQueueStatus), types/api (QueueStatus).
 * Alur         : queueId -> getQueueStatus -> status. Event SSE yang mengubah antrean memicu muat ulang (debounce 300 ms) dan callback onChanged.
 * Catatan      : Dibedakan dari use-queue-board (papan owner). Hook ini hanya membaca status, tidak mengubah apa pun.
 */

import { useCallback, useEffect, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { getQueueStatus } from '@/lib/tickets';
import type { QueueStatus } from '@/types/api';

/** Jeda 300 ms sebelum muat ulang, supaya beberapa event beruntun hanya memicu satu request. */
const RELOAD_DEBOUNCE_MS = 300;

/**
 * Status satu antrian dari sisi customer, diperbarui lewat SSE. `status` null selama muat awal.
 *
 * Argumen:
 * - `queueId`: id antrian yang diamati.
 * - `onChanged`: dipanggil tiap kali event yang relevan datang, sebelum muat ulang selesai. Opsional.
 *
 * Mengembalikan `status` (QueueStatus atau null bila belum dimuat). Field `my_ticket` di dalamnya berisi nomor akun ini.
 */
export function useQueueSpot(queueId: number, onChanged?: () => void) {
  const [status, setStatus] = useState<QueueStatus | null>(null);

  const reload = useCallback(async () => {
    try {
      setStatus(await getQueueStatus(queueId));
    } catch {
      // biarkan status terakhir; SSE akan mencoba lagi saat event berikutnya
    }
  }, [queueId]);

  // Muat awal. then() → setState tidak terjadi sinkron di dalam effect.
  useEffect(() => {
    let active = true;
    void getQueueStatus(queueId).then(
      (data) => {
        if (active) setStatus(data);
      },
      () => {},
    );
    return () => {
      active = false;
    };
  }, [queueId]);

  // Debounce muat ulang. Timer lama dibatalkan agar hanya pemanggilan terakhir yang berjalan.
  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const scheduleReload = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      void reload();
    }, RELOAD_DEBOUNCE_MS);
  }, [reload]);
  // Cleanup saat unmount: timer debounce tidak boleh berjalan setelah komponen hilang.
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  // Event yang mengubah posisi antrean (join, next, skip, batal, hapus, tutup) memicu muat ulang. Pembatalan ikut
  // karena waiting di belakangnya naik satu posisi; tanpa ini posisi customer lain tidak ter-update.
  useQueueEvents(queueId, {
    onEvent: (event) => {
      if (event.type === 'ticket_joined' || event.type === 'ticket_next' || event.type === 'ticket_skipped' || event.type === 'ticket_cancelled' || event.type === 'ticket_removed' || event.type === 'queue_closed') {
        scheduleReload();
        onChanged?.();
      }
    },
    onReconnect: scheduleReload,
    onResync: scheduleReload,
  });

  return status;
}
