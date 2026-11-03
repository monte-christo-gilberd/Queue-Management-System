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

  if (isQrErrorCode(error.code)) return describeQrError(error);

  switch (error.code) {
    case 'SUBSCRIPTION_INACTIVE':
      return 'Langganan Anda tidak aktif. Aktifkan langganan untuk membuat atau mengubah antrian.';
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
