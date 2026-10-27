import { useCallback, useEffect, useMemo, useRef, useState } from 'react';

import { useQueueEvents } from '@/hooks/use-queue-events';
import { ApiError } from '@/lib/api';
import { describeQueueError } from '@/lib/queue-errors';
import { joinByToken, previewJoin } from '@/lib/qr';
import { getQueueStatus } from '@/lib/tickets';
import type { PublicQueue, QueueStatus, Ticket } from '@/types/api';

const RELOAD_DEBOUNCE_MS = 300;

/**
 * Sisi customer untuk satu token join: preview token -> info antrian -> status tiket -> SSE.
 *
 * `token` adalah token join dari QR (`/join?t=<token>`); tanpa token antrian tidak valid.
 * `status` null dan tanpa error = belum bergabung (backend membalas 404 untuk non-pemegang tiket).
 */
export function useCustomerQueue(token: string | null) {
  const invalid = !token;

  const [status, setStatus] = useState<QueueStatus | null>(null);
  const [loading, setLoading] = useState(!invalid);
  const [error, setError] = useState<string>();
  const [joining, setJoining] = useState(false);
  const [joinError, setJoinError] = useState<string>();
  const [info, setInfo] = useState<PublicQueue | null>(null);
  const [infoMissing, setInfoMissing] = useState(invalid);
  const [queueId, setQueueId] = useState<number | null>(null);

  const loadStatus = useCallback(async (id: number) => {
    try {
      const data = await getQueueStatus(id);
      setStatus(data);
      setError(undefined);
      return data;
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        setStatus(null);
        setError(undefined);
        return null;
      }
      setError(describeQueueError(e));
      return null;
    }
  }, []);

  // Preview token -> info publik antrian.
  useEffect(() => {
    if (invalid) return;
    let active = true;
    void previewJoin(token as string)
      .then((data) => {
        if (!active) return;
        setInfo(data);
        setInfoMissing(false);
        setQueueId(data.id);
        setLoading(false);
      })
      .catch((e: unknown) => {
        if (!active) return;
        setInfo(null);
        setInfoMissing(true);
        setError(describeQueueError(e));
        setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [invalid, token]);

  // Status antrian per queueId dari preview. `.then()` agar setState tidak sinkron di effect.
  useEffect(() => {
    if (queueId == null) return;
    let active = true;
    void getQueueStatus(queueId).then(
      (data) => {
        if (!active) return;
        setStatus(data);
        setError(undefined);
        setLoading(false);
      },
      (e: unknown) => {
        if (!active) return;
        if (e instanceof ApiError && e.status === 404) {
          setStatus(null);
          setError(undefined);
        } else {
          setError(describeQueueError(e));
        }
        setLoading(false);
      },
    );
    return () => {
      active = false;
    };
  }, [queueId]);

  const timerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const scheduleReload = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => void loadStatus(queueId as number), RELOAD_DEBOUNCE_MS);
  }, [loadStatus, queueId]);
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  const hasTicket = status?.my_ticket != null;
  useQueueEvents(invalid || !hasTicket ? null : (queueId as number), {
    onEvent: (event) => {
      if (event.type === 'ticket_joined' || event.type === 'ticket_next' || event.type === 'queue_closed') {
        scheduleReload();
      }
    },
    onReconnect: () => {
      scheduleReload();
    },
  });

  const join = useCallback(
    async () => {
      if (!token) return;
      setJoining(true);
      setJoinError(undefined);
      try {
        const ticket: Ticket = await joinByToken(token);
        // Setelah join, status lengkap (nomor dilayani, orang di depan) baru tersedia.
        const data = queueId != null ? await loadStatus(queueId) : null;
        if (data) {
          setStatus(data);
        } else if (info) {
          // Fallback bila status belum bisa dibaca: tampilkan tiket yang baru didapat.
          setStatus({
            queue: info,
            my_ticket: ticket,
            current_number: null,
            waiting_ahead: null,
            previous_number: null,
            next_number: null,
          });
        }
      } catch (e) {
        if (e instanceof ApiError && e.status === 409 && e.code === 'ALREADY_JOINED') {
          // Sudah terdaftar (mis. klik ganda): cukup tampilkan statusnya.
          const data = queueId != null ? await loadStatus(queueId) : null;
          if (data) setStatus(data);
        } else {
          setJoinError(describeQueueError(e));
        }
      } finally {
        setJoining(false);
      }
    },
    [token, queueId, info, loadStatus],
  );

  const invalidMessage = invalid ? 'Tautan tidak valid' : undefined;
  const missingMessage = !invalid && infoMissing && !status ? 'Antrian tidak ditemukan' : undefined;

  return useMemo(
    () => ({
      status,
      info,
      loading,
      error: invalidMessage ?? error ?? missingMessage,
      joining,
      joinError,
      join,
    }),
    [status, info, loading, error, invalidMessage, missingMessage, joining, joinError, join],
  );
}
