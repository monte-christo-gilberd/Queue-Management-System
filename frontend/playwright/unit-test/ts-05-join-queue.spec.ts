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

  test('TC-05-09 Popup Masuk ke Antrian tanpa input tautan manual', async ({ page, request }) => {
    const customer = await registerAccount(request, 'camera');
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();

    await expect(page.getByText('Masuk ke Antrian', { exact: true })).toBeVisible();
    await expect(field(page, 'https://...')).toHaveCount(0);
  });
});
