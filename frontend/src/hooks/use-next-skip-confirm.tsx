/**
 * use-next-skip-confirm: konfirmasi sebelum Next dan Skip di layar owner, lengkap dengan modal yang harus dirender.
 *
 * Dipakai oleh : src/app/(owner)/dashboard.tsx, src/app/(owner)/queues/[queueId]/index.tsx, src/app/(owner)/queues/[queueId]/live.tsx.
 * Bergantung   : components/ui/ConfirmModal (modal), utils/confirm-messages (confirmCopy, tipe ConfirmKind).
 * Alur         : requestNext/requestSkip -> state pending -> modal tampil -> confirm -> onNext atau onSkip.
 * Catatan      : Hook ini ikut merender JSX lewat `modal`, sehingga pemanggil harus menaruh `modal` sekali di layarnya. Tidak ada undo.
 */

import { useCallback, useRef, useState } from 'react';

import { ConfirmModal } from '@/components/ui/ConfirmModal';
import { confirmCopy, type ConfirmKind } from '@/utils/confirm-messages';

/** Argumen hook: data untuk teks konfirmasi dan dua aksi yang dijalankan setelah dikonfirmasi. */
type Options = {
  /** Nomor yang sedang dilayani counter ini, sudah diformat; null bila kosong. */
  current: string | null;
  /** Nama counter, hanya bila antrian punya lebih dari satu counter. */
  counterName?: string | null;
  /** Aksi Next. Dijalankan setelah konfirmasi. Bisa async. */
  onNext: () => void | Promise<void>;
  /** Aksi Skip. Dijalankan setelah konfirmasi. Bisa async. */
  onSkip: () => void | Promise<void>;
};

/**
 * Konfirmasi sebelum Next dan Skip (tidak ada undo). Pakai `requestNext` dan `requestSkip` sebagai pengganti
 * pemanggilan langsung, dan render `modal` sekali di layar. Klik ganda pada tombol konfirmasi hanya menjalankan
 * aksi satu kali: kunci dilepas setelah aksi selesai.
 *
 * Argumen: lihat tipe Options.
 *
 * Mengembalikan objek dengan field:
 * - `requestNext()`: membuka konfirmasi untuk Next.
 * - `requestSkip()`: membuka konfirmasi untuk Skip.
 * - `modal`: elemen modal konfirmasi. Harus dirender di layar.
 */
export function useNextSkipConfirm({ current, counterName, onNext, onSkip }: Options) {
  // Jenis aksi yang menunggu konfirmasi. null berarti modal tertutup.
  const [pending, setPending] = useState<ConfirmKind | null>(null);
  // Ref, bukan state, supaya kunci klik ganda langsung berlaku tanpa menunggu render ulang.
  const running = useRef(false);

  // Modal ditutup dulu, lalu aksi berjalan. Kunci `running` dilepas di finally, jadi klik berikutnya diterima lagi setelah selesai.
  const confirm = useCallback(() => {
    if (pending === null || running.current) return;
    running.current = true;
    const action = pending === 'next' ? onNext : onSkip;
    setPending(null);
    void Promise.resolve(action()).finally(() => {
      running.current = false;
    });
  }, [pending, onNext, onSkip]);

  const copy = confirmCopy(pending ?? 'next', current, counterName);
  const modal = (
    <ConfirmModal
      visible={pending !== null}
      title={copy.title}
      message={copy.message}
      confirmLabel={copy.confirmLabel}
      onConfirm={confirm}
      onCancel={() => setPending(null)}
    />
  );

  return {
    requestNext: () => setPending('next'),
    requestSkip: () => setPending('skip'),
    modal,
  };
}
