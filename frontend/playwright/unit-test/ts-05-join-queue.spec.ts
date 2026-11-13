import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  closeQueueViaApi,
  createQueueViaApi,
  field,
  joinPathViaApi,
  joinTokenOf,
  queueStatusViaApi,
  joinQueueViaApi,
  registerAccount,
  signInThroughUi,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  WEB_URL,
  API_URL,
} from './support';

async function newQueue(request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Join'));
  return { ...queue, ownerToken: session.token };
}

test.describe('TS-05 Bergabung ke antrian (customer)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-05-01 Tautan join di address bar tidak membuat tiket', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'nolink');
    const path = await joinPathViaApi(request, queue.id, queue.ownerToken);
    await signInWithToken(page, customer.token, customer.email, path);

    await expect(page.getByText('Pindai QR di aplikasi')).toBeVisible();
    await expect(action(page, 'Gabung Antrian')).toHaveCount(0);

    const status = await queueStatusViaApi(request, customer.token, queue.id);
    expect(status.status()).toBe(404);
  });

  test('TC-05-02 Tautan join tanpa login meminta login', async ({ page, request }) => {
    const queue = await newQueue(request);
    await page.goto(`${WEB_URL}${await joinPathViaApi(request, queue.id, queue.ownerToken)}`);
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
  });

  test('TC-05-03 Bergabung lewat token memberi nomor di akhir antrian', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'first');
    await joinQueueViaApi(request, customer.token, queue.id, queue.ownerToken);
    await signInWithToken(page, customer.token, customer.email, `/ticket/${queue.id}`);

    await expect(page.getByText('Nomor Anda')).toBeVisible();
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
    await expect(page.getByText('Orang di depan Anda')).toBeVisible();
  });

  test('TC-05-04 Token yang sama tidak membuat nomor baru', async ({ request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'again');
    await joinQueueViaApi(request, customer.token, queue.id, queue.ownerToken);

    const t = joinTokenOf(await joinPathViaApi(request, queue.id, queue.ownerToken));
    const res = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${customer.token}` },
      data: { t },
    });
    expect(res.status()).toBe(200);
    const body = (await res.json()) as { data: { ticket_number?: number; ticket?: { ticket_number: number } } };
    expect(body.data.ticket_number ?? body.data.ticket?.ticket_number).toBe(1);
  });

  test('TC-05-05 Antrian tertutup tidak menerima pendaftaran', async ({ request }) => {
    const queue = await newQueue(request);
    const t = joinTokenOf(await joinPathViaApi(request, queue.id, queue.ownerToken));
    await closeQueueViaApi(request, queue.ownerToken, queue.id);
    const customer = await registerAccount(request, 'closed');

    const res = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${customer.token}` },
      data: { t },
    });
    expect(res.status()).toBe(409);
    expect(((await res.json()) as { error: { code: string } }).error.code).toBe('QUEUE_CLOSED');
  });

  test('TC-05-06 Token tidak valid ditolak API', async ({ request }) => {
    const customer = await registerAccount(request, 'badlink');
    const res = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${customer.token}` },
      data: { t: 'bukan-token' },
    });
    expect(res.status()).toBe(404);
    expect(((await res.json()) as { error: { code: string } }).error.code).toBe('QR_INVALID');
  });

  test('TC-05-07 Tanpa token API menolak', async ({ request }) => {
    const customer = await registerAccount(request, 'notoken');
    const res = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${customer.token}` },
      data: {},
    });
    expect(res.status()).toBeGreaterThanOrEqual(400);
    expect(res.status()).toBeLessThan(500);
  });

  test('TC-05-08 Kode QR statis yang sudah diganti tidak berlaku', async ({ request }) => {
    const queue = await newQueue(request);
    const t = joinTokenOf(await joinPathViaApi(request, queue.id, queue.ownerToken));
    const rep = await request.post(`${API_URL}/api/v1/queues/${queue.id}/qr/replace`, {
      headers: { Authorization: `Bearer ${queue.ownerToken}` },
    });
    expect(rep.status(), await rep.text()).toBe(200);
    const customer = await registerAccount(request, 'oldqr');
    const res = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${customer.token}` },
      data: { t },
    });
    expect(res.status()).toBe(404);
  });

  test('TC-05-09 Popup Masuk ke Antrian menampilkan kamera dan form tautan', async ({ page, request }) => {
    const customer = await registerAccount(request, 'camera');
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();

    await expect(page.getByText('Masuk ke Antrian', { exact: true })).toBeVisible();
    await expect(field(page, 'Token 6 karakter')).toHaveCount(1);
    await expect(action(page, 'Masuk')).toBeVisible();
  });

  test('TC-05-10 Menempel tautan antrian di popup membuat tiket tanpa scan QR', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'paste');
    const path = await joinPathViaApi(request, queue.id, queue.ownerToken);
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();
    await field(page, 'Token 6 karakter').fill(`${WEB_URL}${path}`);
    await action(page, 'Masuk').click();

    await expect(page).toHaveURL(new RegExp(`/ticket/${queue.id}`));
    await expect(page.getByText('Nomor Anda')).toBeVisible();
    expect((await queueStatusViaApi(request, customer.token, queue.id)).status()).toBe(200);
  });

  test('TC-05-11 Tautan yang bukan tautan antrian ditolak dengan pesan', async ({ page, request }) => {
    const customer = await registerAccount(request, 'badlink');
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();
    await field(page, 'Token 6 karakter').fill('https://example.com/lainnya');
    await action(page, 'Masuk').click();

    await expect(page.getByText('Token ini tidak valid.')).toBeVisible();
  });

  // Kode 6 karakter yang diketik di popup (huruf kecil) membuat tiket tanpa memindai QR.
  test('TC-05-12 Mengetik kode 6 karakter di popup membuat tiket tanpa QR', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'typed');
    const path = await joinPathViaApi(request, queue.id, queue.ownerToken);
    const code = joinTokenOf(path);
    expect(code).toMatch(/^[2-9A-HJKMNP-Z]{6}$/);
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();
    await field(page, 'Token 6 karakter').fill(code.toLowerCase());
    await action(page, 'Masuk').click();

    await expect(page).toHaveURL(new RegExp(`/ticket/${queue.id}`));
    await expect(page.getByText('Nomor Anda')).toBeVisible();
  });
});
