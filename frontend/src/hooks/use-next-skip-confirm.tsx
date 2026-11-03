import { useCallback, useRef, useState } from 'react';

import { ConfirmModal } from '@/components/ui/ConfirmModal';
import { confirmCopy, type ConfirmKind } from '@/utils/confirm-messages';

type Options = {
  /** Nomor yang sedang dilayani counter ini, sudah diformat; null bila kosong. */
  current: string | null;
  /** Nama counter, hanya bila antrian punya lebih dari satu counter. */
  counterName?: string | null;
  onNext: () => void | Promise<void>;
  onSkip: () => void | Promise<void>;
};

/**
 * Konfirmasi sebelum Next dan Skip (tidak ada undo). Pakai `requestNext` dan `requestSkip` sebagai pengganti
 * pemanggilan langsung, dan render `modal` sekali di layar. Klik ganda pada tombol konfirmasi hanya menjalankan
 * aksi satu kali: kunci dilepas setelah aksi selesai.
 */
export function useNextSkipConfirm({ current, counterName, onNext, onSkip }: Options) {
  const [pending, setPending] = useState<ConfirmKind | null>(null);
  const running = useRef(false);

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
