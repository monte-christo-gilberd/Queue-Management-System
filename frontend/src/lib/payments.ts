/**
 * payments.ts: sesi checkout Stripe untuk owner dan pemetaan galat pembayaran ke pesan untuk pengguna.
 *
 * Dipakai oleh : app/(owner)/subscription.tsx
 * Bergantung   : lib/api.ts (ApiError, apiFetch)
 * Catatan      : Dilayani payment-service, bukan backend antrian. Nginx meneruskan /api/v1/payments/ ke sana.
 */
import { ApiError, apiFetch } from './api';

/** Checkout: sesi pembayaran Stripe. `checkout_url` dibuka di browser; Stripe mengembalikan user ke /subscription?payment=... */
export type Checkout = {
  /** URL halaman pembayaran Stripe yang dibuka di browser. */
  checkout_url: string;
  /** Waktu sesi checkout kedaluwarsa (ISO, RFC 3339). Setelah itu URL tidak bisa dipakai. */
  expires_at: string;
};

/**
 * POST /api/v1/payments/checkout (butuh token owner). Dilayani payment-service, BUKAN backend antrian: bila
 * service itu mati, hanya pemanggilan ini yang gagal (503 PAYMENT_UNAVAILABLE); bagian aplikasi lain tetap jalan.
 */
export async function startCheckout(): Promise<Checkout> {
  return apiFetch<Checkout>('/api/v1/payments/checkout', { method: 'POST' });
}

/**
 * describePaymentError: mengubah galat membuka pembayaran menjadi pesan untuk pengguna. Fungsi murni.
 * Contoh: ApiError 503 PAYMENT_UNAVAILABLE menghasilkan pesan "Pembayaran sementara tidak tersedia...",
 * sedangkan galat yang bukan ApiError menghasilkan pesan umum.
 */
export function describePaymentError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 429/502/503/504 bisa datang dari nginx tanpa body JSON, jadi dicek lewat status.
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  if (error.code === 'UNKNOWN_USER') return 'Akun tidak ditemukan, silakan login ulang';
  if (
    error.code === 'NETWORK_ERROR' ||
    error.code === 'PAYMENT_UNAVAILABLE' ||
    error.code === 'PAYMENT_PROVIDER_ERROR' ||
    error.status === 502 ||
    error.status === 503 ||
    error.status === 504
  ) {
    return 'Pembayaran sementara tidak tersedia, coba lagi nanti. Antrian Anda tetap berjalan.';
  }
  return 'Terjadi kesalahan di server, coba lagi';
}
