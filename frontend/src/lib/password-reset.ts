import { ApiError, apiFetch } from './api';

/**
 * POST /api/v1/users/forgot-password. Backend selalu menjawab 202 untuk email yang formatnya sah, terdaftar atau
 * tidak, jadi halaman tidak boleh menyiratkan bahwa email itu ada.
 */
export async function requestPasswordReset(email: string): Promise<void> {
  await apiFetch<unknown>('/api/v1/users/forgot-password', { method: 'POST', body: { email } });
}

/** POST /api/v1/users/reset-password. Galat INVALID_TOKEN = tautan tidak sah, kedaluwarsa, atau sudah dipakai. */
export async function resetPassword(token: string, password: string): Promise<void> {
  await apiFetch<unknown>('/api/v1/users/reset-password', { method: 'POST', body: { token, password } });
}

/** Pesan untuk permintaan tautan reset. Fungsi murni. */
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

/** Pesan untuk penyimpanan password baru. Fungsi murni. */
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

/** True bila galat berarti tautan reset sudah tidak bisa dipakai. */
export function isInvalidResetToken(error: unknown): boolean {
  return error instanceof ApiError && error.code === 'INVALID_TOKEN';
}
