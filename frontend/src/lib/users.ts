/**
 * users.ts: panggilan API profil user yang sedang login (me), preferensi notifikasi, dan foto profil.
 *
 * Dipakai oleh : hooks/use-me.ts, hooks/use-avatar-source.ts, app/(owner)/profile/index.tsx,
 *                app/(owner)/subscription.tsx
 * Bergantung   : lib/api.ts (apiFetch, apiUpload, apiDownload, API_BASE_URL)
 * Catatan      : Semua rute /users/me/* butuh login (dijaga middleware auth backend). Foto profil dibaca lewat
 *                apiDownload karena butuh header Authorization.
 */
import { apiDownload, apiFetch, apiUpload, API_BASE_URL } from './api';

/** Me: profil user yang sedang login. is_owner = punya baris di tabel owners. */
export type Me = {
  /** ID user di database. */
  id: number;
  /** Alamat email akun. */
  email: string;
  /** Username akun. Bisa diganti lewat updateMe. */
  username: string;
  /** true bila akun ini owner (punya baris di tabel owners). */
  is_owner: boolean;
  /** Akhir masa langganan (ISO). Hanya owner; null untuk customer. */
  subscription_expires_at: string | null;
  /** Waktu foto profil terakhir diganti (ISO); null bila belum punya foto. Juga pemecah cache. */
  avatar_updated_at: string | null;
  /** Email notifikasi tiket (masuk, batal, dihapus, selesai) menyala. */
  email_notifications: boolean;
  /** Email sudah diverifikasi. Selalu true bila server mematikan verifikasi. false = tampilkan banner di Profil. */
  email_verified: boolean;
};

/** GET /api/v1/users/me (butuh token). */
export async function getMe(): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me');
}

/** PUT /api/v1/users/me: ganti username. Mengembalikan profil terbaru. */
export async function updateMe(username: string): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me', { method: 'PUT', body: { username } });
}

/** PUT /api/v1/users/me/preferences: nyalakan atau matikan email notifikasi tiket. Mengembalikan profil terbaru. */
export async function setEmailNotifications(enabled: boolean): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me/preferences', { method: 'PUT', body: { email_notifications: enabled } });
}

/** Berkas foto untuk diunggah: `File` di web, objek `{ uri, name, type }` di native (bentuk yang dipahami FormData RN). */
export type AvatarUpload = Blob | { uri: string; name: string; type: string };

/** PUT /api/v1/users/me/avatar: ganti foto profil. Server memotong persegi dan memperkecil ke 512x512 JPEG. */
export async function uploadAvatar(file: AvatarUpload, fileName = 'avatar.jpg'): Promise<Me> {
  const form = new FormData();
  // Di RN, FormData menerima objek { uri, name, type }; tipe DOM tidak mengenalnya.
  if ('uri' in file) form.append('file', file as unknown as Blob);
  else form.append('file', file, fileName);
  return apiUpload<Me>('/api/v1/users/me/avatar', form, 'PUT');
}

/** DELETE /api/v1/users/me/avatar: hapus foto profil. Idempoten. */
export async function deleteAvatar(): Promise<void> {
  await apiFetch<void>('/api/v1/users/me/avatar', { method: 'DELETE' });
}

/**
 * avatarUrl: alamat foto profil. Butuh header Authorization, jadi tidak bisa dipakai langsung sebagai `<img src>` di web.
 * `updatedAt` diisi dari Me.avatar_updated_at, sehingga URL berubah setiap foto diganti dan cache browser tidak dipakai.
 */
export function avatarUrl(updatedAt: string): string {
  return `${API_BASE_URL}/api/v1/users/me/avatar?v=${encodeURIComponent(updatedAt)}`;
}

/** GET /api/v1/users/me/avatar sebagai Blob (web: dijadikan object URL). */
export async function fetchAvatarBlob(): Promise<Blob> {
  return apiDownload('/api/v1/users/me/avatar');
}
