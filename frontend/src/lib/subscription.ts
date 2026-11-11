/**
 * subscription.ts: tambah 30 hari langganan owner tanpa pembayaran (MOCK) dan pemetaan galat ke pesan untuk pengguna.
 *
 * Dipakai oleh : app/(owner)/subscription.tsx
 * Bergantung   : lib/api.ts (ApiError, apiFetch)
 * Catatan      : Dilayani backend antrian (bukan payment-service, yang sudah dihapus). Tidak ada Stripe.
 */
import { ApiError, apiFetch } from './api';

/** Hasil penambahan masa langganan: tanggal berakhir yang baru (ISO, RFC 3339). */
export type ExtendResult = {
  subscription_expires_at: string;
};

/**
 * POST /api/v1/owners/subscription/extend (butuh token). Menambah 30 hari langsung. Calon owner otomatis
 * menjadi owner dengan 30 hari pertama.
 */
export async function extendSubscription(): Promise<ExtendResult> {
  return apiFetch<ExtendResult>('/api/v1/owners/subscription/extend', { method: 'POST' });
}

/**
 * describeSubscriptionError: mengubah galat penambahan masa langganan menjadi pesan untuk pengguna. Fungsi murni.
 */
export function describeSubscriptionError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 429/502/503/504 bisa datang dari nginx tanpa body JSON, jadi dicek lewat status.
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  if (error.code === 'UNKNOWN_USER') return 'Akun tidak ditemukan, silakan login ulang';
  if (error.code === 'NETWORK_ERROR' || error.status === 502 || error.status === 503 || error.status === 504) {
    return 'Layanan langganan sementara tidak tersedia, coba lagi nanti. Antrian Anda tetap berjalan.';
  }
  return 'Terjadi kesalahan di server, coba lagi';
}
