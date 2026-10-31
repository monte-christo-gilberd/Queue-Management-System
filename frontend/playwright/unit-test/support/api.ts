import { expect, type APIRequestContext } from '@playwright/test';

import { API_URL, hasSubscribedOwner, SUBSCRIBED_OWNER } from './env';

export const DEFAULT_PASSWORD = 'Password123!';

export type Account = {
  email: string;
  username: string;
  password: string;
  token: string;
};

export function uniqueName(prefix: string): string {
  return `${prefix}${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

/** Mendaftarkan akun baru lewat API. Akun baru belum punya langganan owner. */
export async function registerAccount(request: APIRequestContext, prefix = 'user'): Promise<Account> {
  const username = uniqueName(prefix);
  const email = `${username}@qms-test.local`;
  const password = DEFAULT_PASSWORD;

  const res = await request.post(`${API_URL}/api/v1/users/register`, {
    data: { email, username, password },
  });
  expect(res.status(), await res.text()).toBe(200);
  const body = (await res.json()) as { data: { token: string } };
  return { email, username, password, token: body.data.token };
}

/** Login lewat API (dipakai untuk akun seed, mis. owner berlangganan). */
export async function loginViaApi(
  request: APIRequestContext,
  identifier: string,
  password: string,
): Promise<string> {
  const res = await request.post(`${API_URL}/api/v1/users/login`, {
    data: { identifier, password },
  });
  expect(res.status(), await res.text()).toBe(200);
  const body = (await res.json()) as { data: { token: string } };
  return body.data.token;
}

/** Membuat antrian lewat API (untuk menyiapkan data, bukan untuk diuji). */
export async function createQueueViaApi(
  request: APIRequestContext,
  token: string,
  name: string,
): Promise<{ id: number; queue_name: string; ticket_prefix: string }> {
  const res = await request.post(`${API_URL}/api/v1/queues`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { queue_name: name, queue_description: 'Dibuat oleh tes E2E' },
  });
  expect(res.status(), await res.text()).toBe(201);
  const body = (await res.json()) as { data: { queue: { id: number; queue_name: string; ticket_prefix: string } } };
  return body.data.queue;
}

/** Memanggil NEXT lewat API (dipakai untuk memicu update real-time di sisi customer). */
export async function nextViaApi(request: APIRequestContext, token: string, queueId: number) {
  return request.post(`${API_URL}/api/v1/queues/${queueId}/next`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

/** Memanggil SKIP lewat API (lewati nomor yang dilayani, panggil berikutnya). */
export async function skipViaApi(request: APIRequestContext, token: string, queueId: number) {
  return request.post(`${API_URL}/api/v1/queues/${queueId}/skip`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

/** Mengatur pengaturan Skip antrian lewat API. */
export async function setSkipSettingsViaApi(
  request: APIRequestContext,
  token: string,
  queueId: number,
  body: { fallback: 'remove' | 'back' | 'after_n'; n?: number; max_returns: number },
) {
  return request.put(`${API_URL}/api/v1/queues/${queueId}/settings/skip`, {
    headers: { Authorization: `Bearer ${token}` },
    data: body,
  });
}

/** Daftar tiket antrian (owner) lewat API. */
export async function listTicketsViaApi(request: APIRequestContext, token: string, queueId: number) {
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/tickets`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return res;
}

/** Menutup antrian lewat API (menyiapkan data). */
export async function closeQueueViaApi(request: APIRequestContext, token: string, queueId: number) {
  const res = await request.post(`${API_URL}/api/v1/queues/${queueId}/close`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(res.status(), await res.text()).toBe(200);
}

/** Path join `/join?t=<token>` dari QR owner (GET /queues/:id/qr). */
export async function joinPathViaApi(
  request: APIRequestContext,
  queueId: number,
  ownerToken?: string,
): Promise<string> {
  const token = ownerToken ?? (await subscribedOwnerSession(request))?.token;
  if (!token) throw new Error('owner berlangganan tidak tersedia');
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/qr`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(res.status(), await res.text()).toBe(200);
  const body = (await res.json()) as { data: { join_url: string } };
  const url = new URL(body.data.join_url);
  return `${url.pathname}${url.search}`;
}

/** Token `t` dari path join. */
export function joinTokenOf(joinPath: string): string {
  return new URL(joinPath, 'http://x').searchParams.get('t') ?? '';
}

/** Bergabung lewat token QR (POST /join). Mengembalikan data tiket. */
export async function joinQueueViaApi(
  request: APIRequestContext,
  token: string,
  queueId: number,
  ownerToken?: string,
): Promise<Record<string, unknown>> {
  const t = joinTokenOf(await joinPathViaApi(request, queueId, ownerToken));
  const res = await request.post(`${API_URL}/api/v1/join`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { t },
  });
  expect(res.status(), await res.text()).toBe(200);
  return (await res.json()) as Record<string, unknown>;
}

/** Status antrian untuk pemegang tiket (menyiapkan/memeriksa data). */
export async function queueStatusViaApi(request: APIRequestContext, token: string, queueId: number) {
  return request.get(`${API_URL}/api/v1/queues/${queueId}/status`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

/**
 * Sesi owner berlangganan dari env (QMS_OWNER_EMAIL / QMS_OWNER_PASSWORD).
 * Mengembalikan null bila tidak diset, sehingga tes bisa di-skip.
 */
export async function subscribedOwnerSession(
  request: APIRequestContext,
): Promise<{ token: string; displayName: string } | null> {
  if (!hasSubscribedOwner) return null;
  const token = await loginViaApi(request, SUBSCRIBED_OWNER.email, SUBSCRIBED_OWNER.password);
  return { token, displayName: SUBSCRIBED_OWNER.email };
}

export async function recallViaApi(request: APIRequestContext, token: string, queueId: number) {
  return request.post(`${API_URL}/api/v1/queues/${queueId}/recall`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}
