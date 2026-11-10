/**
 * device-counter: pilihan counter yang dipakai perangkat owner untuk tiap antrian. Fungsi murni kecuali
 * readStoredCounter dan writeStoredCounter yang mengakses localStorage.
 *
 * Dipakai oleh : src/hooks/use-counter-session.ts.
 * Bergantung   : lib/counters.ts (tipe Counter), window.localStorage (web).
 * Catatan      : Pilihan hanya tersimpan di web. Di native, window.localStorage tidak ada, sehingga akses
 *                gagal, diabaikan, dan perangkat jatuh ke counter default.
 */

import type { Counter } from '@/lib/counters';

/** Kunci localStorage pilihan counter per antrian. */
export const DEVICE_COUNTER_KEY_PREFIX = 'qms-device-counter:';

/**
 * Counter yang dipakai perangkat ini (fungsi murni). Pilihan tersimpan dipakai bila counter itu masih ada;
 * bila tidak (belum memilih, atau counter sudah dihapus dari perangkat lain) jatuh ke counter default.
 * Mengembalikan 0 bila daftar counter belum dimuat.
 */
export function resolveDeviceCounter(stored: number | null, counters: Counter[]): number {
  if (stored !== null && counters.some((c) => c.id === stored)) return stored;
  const fallback = counters.find((c) => c.is_default) ?? counters[0];
  return fallback?.id ?? 0;
}

/**
 * Peringatan singkat saat perangkat berpindah dari counter `from`: bila counter itu masih memegang nomor,
 * ia tetap memegangnya dan perangkat ini bisa kembali kapan saja. Null bila counter itu kosong.
 */
export function switchNotice(from: Counter | undefined): string | null {
  if (!from || !from.serving) return null;
  return `${from.name} masih melayani ${from.serving.ticket_number}; pilih ${from.name} lagi untuk melanjutkan.`;
}

/** Pilihan counter tersimpan untuk satu antrian. Galat akses localStorage diabaikan (perangkat tanpa penyimpanan). */
export function readStoredCounter(queueId: number): number | null {
  try {
    const raw = window.localStorage.getItem(`${DEVICE_COUNTER_KEY_PREFIX}${queueId}`);
    if (raw === null) return null;
    const id = Number(raw);
    return Number.isInteger(id) && id > 0 ? id : null;
  } catch {
    return null;
  }
}

/** Simpan `counterId` sebagai pilihan perangkat untuk antrian `queueId`. Galat penyimpanan diabaikan. */
export function writeStoredCounter(queueId: number, counterId: number): void {
  try {
    window.localStorage.setItem(`${DEVICE_COUNTER_KEY_PREFIX}${queueId}`, String(counterId));
  } catch {
    // Penyimpanan tidak tersedia: pilihan hanya berlaku sampai halaman ditutup.
  }
}
