import { apiFetch } from './api';

/** Profil user yang sedang login. is_owner = punya baris di tabel owners. */
export type Me = {
  id: number;
  email: string;
  username: string;
  is_owner: boolean;
  /** Akhir masa langganan (ISO). Hanya owner; null untuk customer. */
  subscription_expires_at: string | null;
};

/** GET /api/v1/users/me (butuh token). */
export async function getMe(): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me');
}

/** PUT /api/v1/users/me: ganti username. Mengembalikan profil terbaru. */
export async function updateMe(username: string): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me', { method: 'PUT', body: { username } });
}
