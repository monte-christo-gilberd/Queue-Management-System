/**
 * subscription.ts: membuat pesanan pembayaran langganan owner (BorderPay lewat backend) dan pemetaan galat ke pesan.
 *
 * Dipakai oleh : app/(user)/subscription.tsx
 * Bergantung   : lib/api.ts (ApiError, apiFetch)
 * Catatan      : Langganan diperpanjang oleh webhook BorderPay setelah pembayaran lunas, bukan oleh aplikasi.
 */
import { ApiError, apiFetch } from './api';

/** Pesanan pembayaran langganan: halaman bayar BorderPay dan ID pesanan. */
export type SubscriptionCheckout = {
  reference_id: string;
  pay_url: string;
  expires_at: string;
  /** Nominal dalam rupiah (ditentukan server). */
  amount: number;
};

/**
 * POST /api/v1/owners/subscription/checkout (butuh token). Membuat pesanan QRIS untuk satu bulan langganan.
 * Frontend membuka pay_url; langganan aktif setelah webhook BorderPay mengonfirmasi pembayaran lunas.
 */
export async function startSubscriptionCheckout(): Promise<SubscriptionCheckout> {
  return apiFetch<SubscriptionCheckout>('/api/v1/owners/subscription/checkout', {
    method: 'POST',
    body: { method: 'qris' },
  });
}

/**
 * describeSubscriptionError: mengubah galat pembuatan pembayaran menjadi pesan untuk pengguna. Fungsi murni.
 */
export function describeSubscriptionError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 429/502/503/504 bisa datang dari nginx tanpa body JSON, jadi dicek lewat status.
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  if (error.code === 'EMAIL_NOT_VERIFIED') return 'Verifikasi email Anda dulu di halaman Profil sebelum berlangganan.';
  if (error.code === 'UNKNOWN_USER') return 'Akun tidak ditemukan, silakan login ulang';
  if (error.code === 'PAYMENT_UNAVAILABLE' || error.code === 'PAYMENT_REJECTED') {
    return 'Pembayaran belum bisa dibuat sekarang, coba lagi nanti. Antrian Anda tetap berjalan.';
  }
  if (error.code === 'NETWORK_ERROR' || error.status === 502 || error.status === 503 || error.status === 504) {
    return 'Layanan langganan sementara tidak tersedia, coba lagi nanti. Antrian Anda tetap berjalan.';
  }
  return 'Terjadi kesalahan di server, coba lagi';
}
