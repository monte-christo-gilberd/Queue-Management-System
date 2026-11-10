/**
 * confirm-messages: teks popup konfirmasi untuk aksi Next dan Skip owner. Fungsi murni.
 *
 * Dipakai oleh : src/hooks/use-next-skip-confirm.tsx.
 * Catatan      : Tidak ada pemanggilan API. Teks ditulis di sini agar mudah diuji (tests/unit/confirm-messages.test.ts).
 */

/** Jenis aksi owner yang meminta konfirmasi sebelum dijalankan. */
export type ConfirmKind = 'next' | 'skip';

/** Teks satu popup konfirmasi. */
export type ConfirmCopy = {
  /** Judul popup, mis. "Lanjut ke nomor berikutnya?". */
  title: string;
  /** Isi popup yang menyebut nomor dan counter yang terpengaruh. */
  message: string;
  /** Label tombol konfirmasi, mis. "Ya, Lanjut". */
  confirmLabel: string;
};

/**
 * Teks popup konfirmasi Next dan Skip (fungsi murni). `current` = nomor yang sedang dilayani counter ini
 * (sudah diformat, null bila kosong). `counterName` hanya diberikan bila antrian punya lebih dari satu counter.
 * Next dan Skip tidak bisa di-undo, jadi popup selalu menyebut nomor yang terpengaruh.
 */
export function confirmCopy(kind: ConfirmKind, current: string | null, counterName?: string | null): ConfirmCopy {
  // Frasa " di <counter>" hanya muncul untuk antrian multi-counter.
  const where = counterName ? ` di ${counterName}` : '';
  if (kind === 'next') {
    return current
      ? {
          title: 'Lanjut ke nomor berikutnya?',
          message: `${current} akan ditandai selesai dilayani${where} dan nomor berikutnya dipanggil.`,
          confirmLabel: 'Ya, Lanjut',
        }
      : {
          title: 'Panggil nomor pertama?',
          message: `Nomor pertama dalam antrian akan dipanggil${where}.`,
          confirmLabel: 'Ya, Lanjut',
        };
  }
  return current
    ? {
        title: `Lewati ${current}?`,
        message: `${current} akan dilewati${where} sesuai pengaturan skip antrian, dan nomor berikutnya dipanggil.`,
        confirmLabel: 'Ya, Lewati',
      }
    : {
        title: 'Lewati nomor?',
        message: `Belum ada nomor yang sedang dilayani${where}.`,
        confirmLabel: 'Ya, Lewati',
      };
}
