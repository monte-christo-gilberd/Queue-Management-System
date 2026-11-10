/**
 * password-reset.ts: panggilan API dan pesan galat untuk lupa dan reset password.
 *
 * Dipakai oleh : app/forgot-password.tsx, app/reset-password.tsx, app/(owner)/profile/index.tsx
 * Bergantung   : lib/api.ts (ApiError, apiFetch)
 * Catatan      : Kedua endpoint publik. requestPasswordReset selalu menjawab 202 (email terdaftar atau tidak), jadi
 *              pesan sukses tidak boleh menyiratkan bahwa email itu ada. Galat 429 bisa datang dari backend atau nginx.
 */
import { ApiError, apiFetch } from './api';

/**
 * POST /api/v1/users/forgot-password. Backend selalu menjawab 202 untuk email yang formatnya sah, terdaftar atau
 * tidak, jadi halaman tidak boleh menyiratkan bahwa email itu ada. Galat khusus: BAD_REQUEST bila format email salah.
 */
export async function requestPasswordReset(email: string): Promise<void> {
  await apiFetch<unknown>('/api/v1/users/forgot-password', { method: 'POST', body: { email } });
}

/**
 * POST /api/v1/users/reset-password. Galat INVALID_TOKEN = tautan tidak sah, kedaluwarsa, atau sudah dipakai.
 * Galat WEAK_PASSWORD bila password terlalu pendek (minimal 8 karakter).
 */
export async function resetPassword(token: string, password: string): Promise<void> {
  await apiFetch<unknown>('/api/v1/users/reset-password', { method: 'POST', body: { token, password } });
}

/**
 * describeForgotError: pesan untuk galat permintaan tautan reset. Fungsi murni.
 * Galat jaringan memakai pesan dari ApiError (sudah berbahasa Indonesia).
 */
export function describeForgotError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 429 datang dari nginx tanpa body JSON, jadi dicek lewat status.
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'BAD_REQUEST':
      return 'Email tidak valid, periksa kembali isian Anda';
    case 'BUSY':
      return 'Server sedang sibuk, coba lagi sebentar';
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}

/** describeResetError: pesan untuk penyimpanan password baru. Fungsi murni. */
export function describeResetError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  if (error.status === 429) return 'Terlalu banyak permintaan, tunggu sebentar lalu coba lagi';
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'WEAK_PASSWORD':
      return 'Password minimal 8 karakter';
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}

/** isInvalidResetToken: true bila galat berarti tautan reset sudah tidak bisa dipakai. */
export function isInvalidResetToken(error: unknown): boolean {
  return error instanceof ApiError && error.code === 'INVALID_TOKEN';
}
