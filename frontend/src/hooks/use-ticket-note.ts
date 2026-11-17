/**
 * use-ticket-note: popup catatan (Append Note) untuk satu nomor tiket di layar owner.
 *
 * Dipakai oleh : src/app/(user)/dashboard.tsx, src/app/(user)/queues/[queueId]/index.tsx.
 * Bergantung   : lib/notes (getTicketNote, saveTicketNote), lib/queue-errors (describeQueueError).
 * Alur         : open(ticketId) -> getTicketNote -> popup terisi. save(text) -> saveTicketNote -> close dan onSaved.
 * Catatan      : Target tiket dikunci saat open, sehingga pindahnya nomor yang dilayani saat popup terbuka tidak mengubah tujuan simpan.
 */

import { useState } from 'react';

import { describeQueueError } from '@/lib/queue-errors';
import { getTicketNote, saveTicketNote } from '@/lib/notes';

/**
 * Catatan satu nomor tiket untuk popup Append Note.
 *
 * Setiap `open` mengambil catatan yang sudah ada dulu, lalu popup terisi dengan catatan itu.
 * `save` membuat catatan bila belum ada, atau menimpanya bila sudah ada (ditentukan backend), lalu memanggil
 * `onSaved` (mis. untuk memuat ulang daftar tiket yang menampilkan catatan).
 * Tiket dikunci saat `open`, jadi pindahnya nomor yang dilayani saat popup terbuka tidak mengubah target.
 *
 * Argumen:
 * - `queueId`: id antrian tempat tiket berada.
 * - `onSaved`: dipanggil setelah catatan tersimpan. Opsional.
 *
 * Mengembalikan objek dengan field:
 * - `visible`: true bila popup terbuka, yaitu saat ada tiket target.
 * - `initialNote`: isi catatan yang sudah ada, atau string kosong.
 * - `hasNote`: true bila tiket sudah punya catatan di server.
 * - `loading`: true selama catatan lama dimuat saat open.
 * - `saving`: true selama request simpan berjalan.
 * - `errorMessage`: pesan galat dari open atau save. undefined bila tidak ada.
 * - `open(ticketId)`: membuka popup untuk tiket itu dan memuat catatannya.
 * - `save(text)`: menyimpan catatan ke tiket target, lalu menutup popup.
 * - `close()`: menutup popup tanpa menyimpan.
 */
export function useTicketNote(queueId: number, onSaved?: () => void) {
  const [ticketId, setTicketId] = useState<number | null>(null);
  const [initialNote, setInitialNote] = useState('');
  const [hasNote, setHasNote] = useState(false);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string>();

  // Popup baru terbuka hanya setelah catatan lama berhasil dimuat. Galat muat tidak membuka popup.
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

  // Tanpa tiket target (popup tertutup) simpan tidak dilakukan. Backend memutuskan apakah membuat catatan baru atau menimpa yang ada.
  async function save(text: string) {
    if (ticketId === null) return;
    setErrorMessage(undefined);
    setSaving(true);
    try {
      await saveTicketNote(queueId, ticketId, text);
      setTicketId(null);
      onSaved?.();
    } catch (e) {
      setErrorMessage(describeQueueError(e));
    } finally {
      setSaving(false);
    }
  }

  /** Menutup popup dan membuang galat, tanpa menyimpan perubahan. */
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
