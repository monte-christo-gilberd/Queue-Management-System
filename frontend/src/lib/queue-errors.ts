import { ApiError } from './api';

/**
 * Pesan untuk aksi antrian (buat, ubah, tutup, lihat detail). Fungsi murni.
 * 402 = langganan owner tidak aktif; ditampilkan sebagai ajakan memperpanjang langganan.
 */
export function describeQueueError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';

  switch (error.code) {
    case 'SUBSCRIPTION_INACTIVE':
      return 'Langganan Anda tidak aktif. Aktifkan langganan untuk membuat atau mengubah antrian.';
    case 'NOT_FOUND':
      return 'Antrian tidak ditemukan';
    case 'QUEUE_EMPTY':
      return 'Tidak ada yang menunggu di antrian ini';
    case 'QUEUE_CLOSED':
      return 'Antrian sudah ditutup';
    case 'ALREADY_JOINED':
      return 'Anda sudah terdaftar di antrian ini';
    case 'RETRY_EXHAUSTED':
      return 'Server sedang sibuk, silakan coba lagi';
    case 'BAD_REQUEST':
      return 'Data tidak valid, periksa kembali isian Anda';
    case 'QR_INVALID':
      return 'Tautan QR tidak valid atau sudah tidak berlaku. Pindai QR terbaru dari pemilik antrian.';
    case 'QR_EXPIRED':
      return 'Tautan QR sudah kedaluwarsa. Pindai ulang QR terbaru.';
    case 'QR_UNAVAILABLE':
      return 'Layanan QR sedang tidak tersedia, coba lagi';
    case 'QR_TYPE_INVALID':
      return 'Mode QR tidak dikenal. Pilih statis atau dinamis.';
    case 'QR_REPLACE_NOT_ALLOWED':
      return 'QR dinamis tidak bisa diganti; biarkan kedaluwarsa dan diperbarui otomatis.';
    case 'NETWORK_ERROR':
      return error.message;
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}
