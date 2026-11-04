import { ApiError, apiFetch } from './api';

/** Sesi pembayaran Stripe. `checkout_url` dibuka di browser; Stripe mengembalikan user ke /subscription?payment=... */
export type Checkout = {
  checkout_url: string;
  expires_at: string;
};

/**
 * POST /api/v1/payments/checkout (butuh token owner). Dilayani payment-service, BUKAN backend antrian: bila
 * service itu mati, hanya pemanggilan ini yang gagal (503 PAYMENT_UNAVAILABLE); bagian aplikasi lain tetap jalan.
 */
export async function startCheckout(): Promise<Checkout> {
  return apiFetch<Checkout>('/api/v1/payments/checkout', { method: 'POST' });
}

/** Pesan untuk kegagalan membuka pembayaran. Fungsi murni. */
export function describePaymentError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 429/502/503/504 bisa datang dari nginx tanpa body JSON, jadi dicek lewat status.
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  if (error.code === 'NOT_OWNER') return 'Hanya owner yang dapat memperpanjang langganan';
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
