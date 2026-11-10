/**
 * email-verification.ts: panggilan API dan pesan galat untuk verifikasi email.
 *
 * Dipakai oleh : app/verify-email.tsx, app/(owner)/profile/index.tsx
 * Bergantung   : lib/api.ts (ApiError, apiFetch)
 * Catatan      : verifyEmail bersifat publik (token di tautan email). resendVerification butuh login.
 *                Galat 429 bisa datang dari backend (TOO_MANY_REQUESTS) atau nginx (tanpa body JSON).
 */
import { ApiError, apiFetch } from './api';

/**
 * POST /api/v1/users/verify-email (publik; kredensialnya token di tautan email).
 * Galat INVALID_TOKEN = tautan tidak sah, kedaluwarsa, atau sudah dipakai.
 */
export async function verifyEmail(token: string): Promise<void> {
  await apiFetch<unknown>('/api/v1/users/verify-email', { method: 'POST', body: { token } });
}

/**
 * POST /api/v1/users/me/verify-email/resend (butuh login). Backend menjawab 202 juga bila akun sudah terverifikasi
 * (tidak ada yang dikirim). Batas 3 email per jam per akun: galat TOO_MANY_REQUESTS.
 */
export async function resendVerification(): Promise<void> {
  await apiFetch<unknown>('/api/v1/users/me/verify-email/resend', { method: 'POST' });
}

/** describeResendError: pesan untuk permintaan kirim ulang email verifikasi. Fungsi murni. */
export function describeResendError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 429 bisa datang dari backend (kode TOO_MANY_REQUESTS) atau nginx (tanpa body JSON), jadi dicek lewat status.
  if (error.status === 429) return 'Terlalu banyak permintaan, coba lagi sekitar satu jam lagi';
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    default:
      return 'Email verifikasi tidak bisa dikirim sekarang, coba lagi';
  }
}

/** describeVerifyError: pesan untuk halaman verifikasi saat galat BUKAN tautan tidak sah (jaringan atau server). Fungsi murni. */
export function describeVerifyError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  if (error.code === 'NETWORK_ERROR') return error.message;
  return 'Terjadi kesalahan di server, coba lagi';
}

/** isInvalidVerifyToken: true bila galat berarti tautan verifikasi sudah tidak bisa dipakai. */
export function isInvalidVerifyToken(error: unknown): boolean {
  return error instanceof ApiError && error.code === 'INVALID_TOKEN';
}

/** resendSuccessMessage: pesan sukses setelah email kirim ulang diminta. */
export function resendSuccessMessage(email: string): string {
  return `Tautan verifikasi sudah dikirim ke ${email}`;
}
