/**
 * queue-errors.ts: pesan untuk galat aksi antrian, counter, dan sisip customer.
 *
 * Dipakai oleh : app/(owner)/dashboard.tsx, app/(owner)/queues/[queueId]/index.tsx, hooks/use-queue-board.ts,
 *                hooks/use-ticket-queue.ts, hooks/use-counter-session.ts, hooks/use-queues.ts,
 *                hooks/use-skip-settings.ts, hooks/use-history.ts, hooks/use-ticket-note.ts,
 *                components/ui/JoinQueueModals.tsx.
 * Bergantung   : lib/api.ts (ApiError), lib/qr-errors.ts (kode berawalan QR_).
 * Catatan      : pesan ditujukan ke owner yang sedang bekerja di layar, bukan ke customer.
 */
import { ApiError } from './api';
import { describeQrError, isQrErrorCode } from './qr-errors';

/** Pesan untuk aksi counter (buat, ganti nama, hapus). Kode lain jatuh ke pesan antrian umum. */
export function describeCounterError(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'DUPLICATE_COUNTER_NAME':
        return 'Nama counter sudah dipakai';
      case 'COUNTER_LIMIT':
        return 'Maksimal 10 counter per antrian';
      case 'DEFAULT_COUNTER':
        return 'Counter default tidak bisa dihapus';
      case 'QUEUE_CLOSED':
        return 'Antrian sudah ditutup';
    }
  }
  return describeQueueError(error);
}

/**
 * Pesan untuk sisip customer (US-10). Kalimatnya ditujukan ke owner, bukan customer:
 * ALREADY_JOINED dan NOT_FOUND di sini berarti akun yang dicari, bukan user yang sedang login.
 */
export function describeInsertError(error: unknown): string {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'NOT_FOUND':
        return 'Akun tidak ditemukan. Periksa email atau username.';
      case 'AMBIGUOUS_ACCOUNT':
        return 'Lebih dari satu akun memakai username itu. Gunakan emailnya.';
      case 'ALREADY_JOINED':
        return 'Akun ini sudah ikut antrian.';
    }
  }
  return describeQueueError(error);
}

/**
 * Pesan untuk aksi antrian (buat, ubah, tutup, lihat detail). Fungsi murni.
 * 402 = langganan owner tidak aktif; ditampilkan sebagai ajakan memperpanjang langganan.
 */
export function describeQueueError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';

  // Kode berawalan QR_ diterjemahkan oleh qr-errors.ts.
  if (isQrErrorCode(error.code)) return describeQrError(error);

  switch (error.code) {
    case 'SUBSCRIPTION_INACTIVE':
      return 'Langganan Anda tidak aktif. Aktifkan langganan untuk membuat atau mengubah antrian.';
    case 'EMAIL_NOT_VERIFIED':
      return 'Verifikasi email Anda dulu di halaman Profil untuk bergabung atau membuat antrian.';
    case 'NOT_FOUND':
      return 'Antrian tidak ditemukan';
    case 'QUEUE_EMPTY':
      return 'Tidak ada yang menunggu di antrian ini';
    case 'NOTHING_TO_SKIP':
      return 'Belum ada nomor yang sedang dilayani';
    case 'NOTHING_TO_RECALL':
      return 'Belum ada nomor yang sedang dilayani';
    case 'RECALL_TOO_SOON':
      return 'Panggil ulang tersedia 3 detik setelah nomor dipanggil';
    case 'COUNTER_STATE_CHANGED':
      return 'Counter ini baru saja dilanjutkan dari perangkat lain';
    case 'ALREADY_RECALLED':
      return 'Nomor ini sudah dipanggil ulang';
    case 'QUEUE_CLOSED':
      return 'Antrian sudah ditutup';
    case 'ALREADY_JOINED':
      return 'Anda sudah terdaftar di antrian ini';
    case 'RETRY_EXHAUSTED':
      return 'Server sedang sibuk, silakan coba lagi';
    case 'BAD_REQUEST':
      return 'Data tidak valid, periksa kembali isian Anda';
    case 'NETWORK_ERROR':
      return error.message;
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}
