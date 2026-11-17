/**
 * profile: validasi nama pengguna dan foto profil, status langganan, dan pemetaan galat profil. Fungsi murni
 * kecuali describeProfileError yang hanya membaca ApiError.
 *
 * Dipakai oleh : src/app/(user)/profile/index.tsx, src/app/(user)/subscription.tsx, src/components/Profile.tsx.
 * Bergantung   : lib/api.ts (ApiError), utils/format.ts (formatDate).
 * Alur         : PUT /api/v1/users/me (ubah nama) dan PUT /api/v1/users/me/avatar (foto) memakai helper ini
 *                untuk validasi awal dan pesan galat.
 * Catatan      : Aturan nama dan foto mengikuti backend. Batas foto 5 MB ditegakkan backend; nginx mengizinkan 6 MB.
 */

import { ApiError } from '@/lib/api';

import { formatDate } from './format';

/** Panjang maksimal nama pengguna dalam karakter (sama dengan backend). */
export const USERNAME_MAX = 50;

/** Pesan galat untuk username yang tidak boleh disimpan, atau null bila boleh. Aturan sama dengan backend. */
export function validateUsername(raw: string): string | null {
  const value = raw.trim();
  if (value.length === 0) return 'Nama tidak boleh kosong';
  if ([...value].length > USERNAME_MAX) return `Nama maksimal ${USERNAME_MAX} karakter`;
  // Backend memakai aturan no_html_tags; pemeriksaan < dan > di sini meniru aturan itu dari sisi klien.
  if (/[<>]/.test(value)) return 'Nama tidak boleh mengandung karakter < atau >';
  return null;
}

/** Status langganan untuk kartu profil. expiresAt null = belum pernah berlangganan. */
export function subscriptionStatus(
  expiresAt: string | null,
  now: Date = new Date(),
): { active: boolean; description: string } {
  if (!expiresAt) {
    return { active: false, description: 'Anda belum berlangganan atau belum membayar tagihan bulan ini.' };
  }
  // Aktif hanya bila tanggal berakhir masih setelah `now`; `now` jadi parameter supaya bisa diuji.
  const end = new Date(expiresAt);
  if (end.getTime() > now.getTime()) {
    return { active: true, description: `Aktif hingga ${formatDate(expiresAt)}` };
  }
  return { active: false, description: `Langganan berakhir ${formatDate(expiresAt)}` };
}

/** Pesan untuk galat dari GET/PUT /users/me. */
export function describeProfileError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  // 413 dan 415 bisa datang dari nginx tanpa body JSON, jadi dicek lewat status.
  if (error.status === 413) return 'Ukuran foto terlalu besar (maksimal 5 MB)';
  if (error.status === 415) return 'Foto harus berformat JPEG, PNG, atau WebP';
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'BAD_REQUEST':
      return error.message.toLowerCase().includes('image')
        ? 'Foto tidak bisa dibaca, coba foto lain'
        : 'Nama tidak valid, periksa kembali isian Anda';
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}

/** Batas ukuran foto sebelum diunggah; sama dengan backend (imageproc.MaxUploadBytes). Nginx mengizinkan 6 MB untuk rute avatar. */
export const AVATAR_MAX_BYTES = 5 * 1024 * 1024;

/** Pesan galat bila foto yang dipilih jelas tidak bisa diterima, atau null. `size` boleh tidak diketahui. */
export function validateAvatarPick(size: number | null | undefined, mimeType?: string | null): string | null {
  if (mimeType && !/^image\/(jpeg|png|webp)$/i.test(mimeType)) return 'Foto harus berformat JPEG, PNG, atau WebP';
  if (typeof size === 'number' && size > AVATAR_MAX_BYTES) return 'Ukuran foto terlalu besar (maksimal 5 MB)';
  return null;
}
