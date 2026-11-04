import { ApiError } from './api';

/**
 * Pesan untuk form login/register dari error backend. Fungsi murni.
 * Kode error mengikuti kontrak backend (FRONTEND_GENERATION_GUIDE.md bagian 3).
 */
export function describeAuthError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';

  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'UNAUTHORIZED':
      return 'Email/username atau password salah';
    case 'CONFLICT':
      return 'Email sudah terdaftar';
    case 'WEAK_PASSWORD':
      return 'Password minimal 8 karakter';
    case 'BAD_REQUEST':
      return 'Data tidak valid, periksa kembali isian Anda';
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}
