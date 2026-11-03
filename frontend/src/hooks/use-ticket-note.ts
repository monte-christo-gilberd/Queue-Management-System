import { useState } from 'react';

import { describeQueueError } from '@/lib/queue-errors';
import { getTicketNote, saveTicketNote } from '@/lib/notes';

/**
 * Catatan satu nomor tiket untuk popup Append Note.
 *
 * Setiap `open` mengambil catatan yang sudah ada dulu, lalu popup terisi dengan catatan itu.
 * `save` membuat catatan bila belum ada, atau menimpanya bila sudah ada (ditentukan backend).
 * Tiket dikunci saat `open`, jadi pindahnya nomor yang dilayani saat popup terbuka tidak mengubah target.
 */
export function useTicketNote(queueId: number) {
  const [ticketId, setTicketId] = useState<number | null>(null);
  const [initialNote, setInitialNote] = useState('');
  const [hasNote, setHasNote] = useState(false);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  async function open(targetTicketId: number) {
    setErrorMessage(undefined);
    setLoading(true);
    try {
      const existing = await getTicketNote(queueId, targetTicketId);
      setTicketId(targetTicketId);
      setInitialNote(existing.note ?? '');
      setHasNote(existing.note !== null);
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setLoading(false);
    }
  }

  async function save(text: string) {
    if (ticketId === null) return;
    setErrorMessage(undefined);
    setSaving(true);
    try {
      await saveTicketNote(queueId, ticketId, text);
      setTicketId(null);
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setSaving(false);
    }
  }

  function close() {
    setTicketId(null);
    setErrorMessage(undefined);
  }

  return {
    visible: ticketId !== null,
    initialNote,
    hasNote,
    loading,
    saving,
    errorMessage,
    open,
    save,
    close,
  };
}
