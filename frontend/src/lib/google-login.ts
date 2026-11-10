/**
 * google-login.ts: pemanggilan API login Google dan penautan akun (fungsi murni terhadap apiFetch; dapat diuji di Node).
 *
 * Dipakai oleh : store/authStore.ts (loginWithGoogle, linkGoogle).
 * Bergantung   : lib/api.ts (apiFetch, ApiError), types/api.ts (AuthResponse).
 * Alur         : klien mendapat ID token dari Google -> exchangeGoogleIdToken -> token QMS, ATAU 409 GOOGLE_LINK_REQUIRED
 *                dengan link_token (email sudah terdaftar dengan password) -> layar konfirmasi password ->
 *                confirmGoogleLink(linkToken, password) -> token QMS.
 * Catatan      : link_token hanya hidup 5 menit dan sekali pakai; simpan di memori, jangan di storage, dan jangan dicatat di log.
 */
import { ApiError, apiFetch } from '@/lib/api';
import type { AuthResponse } from '@/types/api';

/** GoogleExchangeResult: hasil menukar ID token Google. */
export type GoogleExchangeResult =
  | { kind: 'token'; token: string }
  /** Email sudah terdaftar dengan password: pengguna harus membuktikan password-nya lebih dulu. */
  | { kind: 'link_required'; linkToken: string };

/**
 * exchangeGoogleIdToken menukar ID token Google dengan token QMS (POST /api/v1/users/google).
 * 409 GOOGLE_LINK_REQUIRED diubah menjadi `link_required` (bukan galat); galat lain dilempar sebagai ApiError.
 */
export async function exchangeGoogleIdToken(idToken: string): Promise<GoogleExchangeResult> {
  try {
    const { token } = await apiFetch<AuthResponse>('/api/v1/users/google', {
      method: 'POST',
      body: { id_token: idToken },
    });
    return { kind: 'token', token };
  } catch (e) {
    if (e instanceof ApiError && e.status === 409 && e.code === 'GOOGLE_LINK_REQUIRED') {
      const linkToken = e.details?.link_token;
      if (typeof linkToken === 'string' && linkToken) return { kind: 'link_required', linkToken };
    }
    throw e;
  }
}

/** confirmGoogleLink menautkan akun Google setelah password dibuktikan (POST /api/v1/users/google/link); mengembalikan token QMS. */
export async function confirmGoogleLink(linkToken: string, password: string): Promise<string> {
  const { token } = await apiFetch<AuthResponse>('/api/v1/users/google/link', {
    method: 'POST',
    body: { link_token: linkToken, password },
  });
  return token;
}

/** isLinkSessionOver: true bila galat berarti link token tidak bisa dipakai lagi (habis, terbakar, atau sudah tertaut). */
export function isLinkSessionOver(error: unknown): boolean {
  return error instanceof ApiError && (error.code === 'INVALID_TOKEN' || error.code === 'CONFLICT');
}
