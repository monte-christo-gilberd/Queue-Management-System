import { apiDownload, apiFetch, apiUpload, API_BASE_URL } from './api';

/** Profil user yang sedang login. is_owner = punya baris di tabel owners. */
export type Me = {
  id: number;
  email: string;
  username: string;
  is_owner: boolean;
  /** Akhir masa langganan (ISO). Hanya owner; null untuk customer. */
  subscription_expires_at: string | null;
  /** Waktu foto profil terakhir diganti (ISO); null bila belum punya foto. Juga pemecah cache. */
  avatar_updated_at: string | null;
};

/** GET /api/v1/users/me (butuh token). */
export async function getMe(): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me');
}

/** PUT /api/v1/users/me: ganti username. Mengembalikan profil terbaru. */
export async function updateMe(username: string): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me', { method: 'PUT', body: { username } });
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

/** Alamat foto profil. Butuh header Authorization, jadi tidak bisa dipakai langsung sebagai `<img src>` di web. */
export function avatarUrl(updatedAt: string): string {
  return `${API_BASE_URL}/api/v1/users/me/avatar?v=${encodeURIComponent(updatedAt)}`;
}

/** GET /api/v1/users/me/avatar sebagai Blob (web: dijadikan object URL). */
export async function fetchAvatarBlob(): Promise<Blob> {
  return apiDownload('/api/v1/users/me/avatar');
}
