/**
 * auth-errors.ts: pesan untuk galat form login dan register.
 *
 * Dipakai oleh : app/login.tsx (login), app/register.tsx (register).
 * Bergantung   : lib/api.ts (ApiError).
 */
import { ApiError } from './api';

/** Pesan saat layanan login tidak tersedia. Menjelaskan bahwa user yang sudah masuk tidak terdampak. */
export const AUTH_UNAVAILABLE_MESSAGE =
  'Login sedang tidak tersedia, coba lagi sebentar lagi. Jika Anda sudah masuk sebelumnya, aplikasi tetap bisa dipakai.';

/**
 * Pesan untuk form login/register dari error backend. Fungsi murni.
 * Kode error mengikuti kontrak backend (FRONTEND_GENERATION_GUIDE.md bagian 3).
 */
export function describeAuthError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';

  // auth-service mati: nginx membalas 503 AUTH_UNAVAILABLE. 502/504 tanpa body JSON (kode UNKNOWN) juga berarti
  // layanan login tidak terjangkau. Bukan salah user, dan bukan sesi habis: user yang sudah login tetap bisa memakai
  // aplikasi, karena backend memverifikasi token secara lokal.
  if (error.code === 'AUTH_UNAVAILABLE' || error.status === 502 || error.status === 504) {
    return AUTH_UNAVAILABLE_MESSAGE;
  }

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

/**
 * Pesan galat untuk login dengan Google (setelah ID token didapat dan dikirim ke backend). Fungsi murni.
 * 409 `GOOGLE_LINK_REQUIRED` BUKAN galat bagi pengguna (diarahkan ke layar konfirmasi password), jadi tidak dipetakan di sini.
 */
export function describeGoogleError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Login dengan Google gagal, coba lagi';
  if (error.code === 'AUTH_UNAVAILABLE' || error.status === 502 || error.status === 504) {
    return AUTH_UNAVAILABLE_MESSAGE;
  }
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'GOOGLE_UNAVAILABLE':
      return 'Login dengan Google sedang tidak tersedia, coba lagi nanti';
    case 'GOOGLE_EMAIL_UNVERIFIED':
      return 'Email akun Google Anda belum diverifikasi oleh Google';
    case 'CONFLICT':
      return 'Email ini sudah tertaut ke akun Google yang lain';
    case 'UNAUTHORIZED':
    case 'BAD_REQUEST':
      return 'Login dengan Google gagal, coba lagi';
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}

/**
 * Pesan galat untuk layar konfirmasi password penautan akun Google. Fungsi murni.
 * `INVALID_TOKEN` berarti sesi penautan habis (5 menit) atau terlalu sering salah: layar harus mengulang dari awal.
 */
export function describeLinkError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  if (error.code === 'AUTH_UNAVAILABLE' || error.status === 502 || error.status === 504) {
    return AUTH_UNAVAILABLE_MESSAGE;
  }
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'UNAUTHORIZED':
      return 'Password salah';
    case 'INVALID_TOKEN':
      return 'Sesi penautan habis atau terlalu banyak percobaan. Silakan login dengan Google lagi';
    case 'CONFLICT':
      return 'Akun ini sudah tertaut ke akun Google yang lain';
    case 'BAD_REQUEST':
      return 'Password wajib diisi';
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}
