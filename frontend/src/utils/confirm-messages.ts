/** Jenis aksi owner yang meminta konfirmasi sebelum dijalankan. */
export type ConfirmKind = 'next' | 'skip';

export type ConfirmCopy = { title: string; message: string; confirmLabel: string };

/**
 * Teks popup konfirmasi Next dan Skip (fungsi murni). `current` = nomor yang sedang dilayani counter ini
 * (sudah diformat, null bila kosong). `counterName` hanya diberikan bila antrian punya lebih dari satu counter.
 * Next dan Skip tidak bisa di-undo, jadi popup selalu menyebut nomor yang terpengaruh.
 */
export function confirmCopy(kind: ConfirmKind, current: string | null, counterName?: string | null): ConfirmCopy {
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
