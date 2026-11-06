import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  createQueueViaApi,
  joinPathViaApi,
  joinTokenOf,
  mailpitAvailable,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  verificationTokenOf,
  waitForMail,
  WEB_URL,
} from './support';

const SUBJECT = /verifikasi/i;

async function me(request: APIRequestContext, token: string) {
  const res = await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(token) });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).data as { email_verified: boolean };
}

async function verifyToken(request: APIRequestContext, token: string) {
  return request.post(`${API_URL}/api/v1/users/verify-email`, { data: { token } });
}

async function resend(request: APIRequestContext, token: string) {
  return request.post(`${API_URL}/api/v1/users/me/verify-email/resend`, { headers: auth(token) });
}

async function errorCode(res: { json(): Promise<unknown> }): Promise<string> {
  return ((await res.json()) as { error?: { code?: string } }).error?.code ?? '';
}

/** Token dari email verifikasi terbaru ke `email`. */
async function tokenFromMail(request: APIRequestContext, email: string): Promise<string> {
  const mail = await waitForMail(request, email, { subject: SUBJECT });
  const token = verificationTokenOf(mail);
  if (!token) throw new Error(`email tanpa tautan verifikasi: ${mail.text}`);
  return token;
}

test.describe('TS-33 Verifikasi email', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    testInfo.skip(!(await mailpitAvailable(request)), 'Mailpit tidak berjalan (docker compose up mailpit).');
    // Stack tanpa verifikasi menyatakan akun baru terverifikasi sejak awal; tes ini tidak berlaku di sana.
    const probe = await registerAccount(request, 'probe', { verify: false });
    testInfo.skip((await me(request, probe.token)).email_verified, 'Verifikasi email mati di stack ini.');
  });

  test('TC-33-01 Akun baru belum terverifikasi dan menerima email berisi tautan', async ({ request }) => {
    const acc = await registerAccount(request, 'ver', { verify: false });

    expect((await me(request, acc.token)).email_verified).toBe(false);
    const mail = await waitForMail(request, acc.email, { subject: SUBJECT });
    expect(mail.links.some((l) => l.includes('/verify-email?token='))).toBe(true);
    expect(mail.text).toContain(acc.username);
  });

  test('TC-33-02 Akun belum terverifikasi tidak bisa membuat antrian', async ({ request }) => {
    const acc = await registerAccount(request, 'ver', { verify: false });

    const res = await request.post(`${API_URL}/api/v1/queues`, {
      headers: auth(acc.token),
      data: { queue_name: 'Tidak boleh', queue_description: '' },
    });

    expect(res.status()).toBe(403);
    expect(await errorCode(res)).toBe('EMAIL_NOT_VERIFIED');
  });

  test('TC-33-03 Akun belum terverifikasi tidak bisa bergabung, sesudah verifikasi bisa', async ({ request }) => {
    const owner = await subscribedOwnerSession(request);
    test.skip(owner === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
    if (!owner) return;
    const queue = await createQueueViaApi(request, owner.token, `Verifikasi ${Date.now()}`);
    const t = joinTokenOf(await joinPathViaApi(request, queue.id, owner.token));
    const acc = await registerAccount(request, 'ver', { verify: false });

    const blocked = await request.post(`${API_URL}/api/v1/join`, { headers: auth(acc.token), data: { t } });
    expect(blocked.status()).toBe(403);
    expect(await errorCode(blocked)).toBe('EMAIL_NOT_VERIFIED');

    expect((await verifyToken(request, await tokenFromMail(request, acc.email))).status()).toBe(200);
    expect((await me(request, acc.token)).email_verified).toBe(true);
    const ok = await request.post(`${API_URL}/api/v1/join`, { headers: auth(acc.token), data: { t } });
    expect(ok.status(), await ok.text()).toBe(200);
  });

  test('TC-33-04 Tautan hanya bisa dipakai sekali dan token asal-asalan ditolak', async ({ request }) => {
    const acc = await registerAccount(request, 'ver', { verify: false });
    const token = await tokenFromMail(request, acc.email);

    expect((await verifyToken(request, token)).status()).toBe(200);

    const again = await verifyToken(request, token);
    expect(again.status()).toBe(400);
    expect(await errorCode(again)).toBe('INVALID_TOKEN');
    const bogus = await verifyToken(request, 'bukan-token');
    expect(bogus.status()).toBe(400);
    expect(await errorCode(bogus)).toBe('INVALID_TOKEN');
  });

  test('TC-33-05 Kirim ulang membatalkan tautan lama dan dibatasi 3 email per jam', async ({ request }) => {
    const acc = await registerAccount(request, 'ver', { verify: false });
    const first = await tokenFromMail(request, acc.email);

    expect((await resend(request, acc.token)).status()).toBe(202);
    expect((await resend(request, acc.token)).status()).toBe(202);

    const limited = await resend(request, acc.token);
    expect(limited.status()).toBe(429);
    expect(await errorCode(limited)).toBe('TOO_MANY_REQUESTS');
    expect((await verifyToken(request, first)).status(), 'tautan lama sudah dibatalkan').toBe(400);
  });

  test('TC-33-06 Kirim ulang butuh login', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/v1/users/me/verify-email/resend`);
    expect(res.status()).toBe(401);
  });

  test('TC-33-07 Profil menampilkan banner, tombol kirim ulang, dan banner hilang setelah verifikasi', async ({ page, request }) => {
    const acc = await registerAccount(request, 'ver', { verify: false });
    const first = await tokenFromMail(request, acc.email);
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await expect(page.getByText('Email belum diverifikasi', { exact: true }).first()).toBeVisible();
    await action(page, 'Kirim Ulang Email Verifikasi').click();
    await expect(page.getByText(`Tautan verifikasi sudah dikirim ke ${acc.email}`)).toBeVisible();

    // Kirim ulang membatalkan tautan pertama; email kedua membawa tautan yang berlaku.
    await expect.poll(async () => (await tokenFromMail(request, acc.email)) !== first).toBe(true);
    const latestToken = await tokenFromMail(request, acc.email);

    await page.goto(`${WEB_URL}/verify-email?token=${encodeURIComponent(latestToken)}`);
    await expect(page.getByText('Email Terverifikasi', { exact: true }).first()).toBeVisible();

    await signInWithToken(page, acc.token, acc.username, '/profile');
    await expect(page.getByText('Email belum diverifikasi', { exact: true })).toHaveCount(0);
  });

  test('TC-33-08 Halaman verifikasi dengan tautan tidak sah menampilkan pesan, tanpa perlu login', async ({ page }) => {
    await page.goto(`${WEB_URL}/verify-email?token=tidak-sah`);
    await expect(page.getByText('Tautan Tidak Berlaku', { exact: true }).first()).toBeVisible();

    await page.goto(`${WEB_URL}/verify-email`);
    await expect(page.getByText('Tautan Tidak Berlaku', { exact: true }).first()).toBeVisible();
  });
});
