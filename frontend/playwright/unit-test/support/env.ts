/** Alamat dan kredensial uji. Semua bisa diganti lewat variabel lingkungan. */
export const WEB_URL = process.env.QMS_WEB_URL ?? 'http://localhost:8081';
export const API_URL = process.env.QMS_API_URL ?? 'http://localhost:8080';

/**
 * Owner dengan langganan aktif. Dibuat manual di database (belum ada endpoint langganan):
 *   INSERT INTO owners (user_id, subscription_expires_at) VALUES (<id user>, now() + interval '30 days');
 * Tes yang membutuhkannya di-skip bila variabel ini kosong.
 */
export const SUBSCRIBED_OWNER = {
  email: process.env.QMS_OWNER_EMAIL ?? '',
  password: process.env.QMS_OWNER_PASSWORD ?? '',
};

export const hasSubscribedOwner = SUBSCRIBED_OWNER.email !== '' && SUBSCRIBED_OWNER.password !== '';

/** Kunci penyimpanan sesi di web (AsyncStorage → localStorage). */
export const AUTH_STORAGE_KEY = 'qms-auth';

/**
 * Owner dengan langganan yang sudah kedaluwarsa (opsional, disiapkan di DB).
 * Dipakai TC-04-03; tes di-skip bila kosong.
 */
export const EXPIRED_OWNER = {
  email: process.env.QMS_EXPIRED_OWNER_EMAIL ?? 'owner3@test.com',
  password: process.env.QMS_EXPIRED_OWNER_PASSWORD ?? 'password',
};

export const hasExpiredOwner = EXPIRED_OWNER.email !== '' && EXPIRED_OWNER.password !== '';
