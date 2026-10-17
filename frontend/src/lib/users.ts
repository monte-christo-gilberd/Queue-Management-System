import { apiFetch } from './api';

/** Profil user yang sedang login. is_owner = punya baris di tabel owners. */
export type Me = {
  id: number;
  email: string;
  username: string;
  is_owner: boolean;
};

/** GET /api/v1/users/me (butuh token). */
export async function getMe(): Promise<Me> {
  return apiFetch<Me>('/api/v1/users/me');
}
