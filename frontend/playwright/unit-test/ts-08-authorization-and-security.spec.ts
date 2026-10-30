import { expect, test } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  field,
  joinQueueViaApi,
  queueStatusViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  API_URL,
} from './support';

test.describe('TS-08 Otorisasi dan keamanan akses', () => {
  test('TC-08-01 Orang tanpa tiket tidak bisa melihat status antrian', async ({ page, request }) => {
    const session = await subscribedOwnerSession(request);
    test.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD.');
    if (!session) return;
    const queue = await createQueueViaApi(request, session.token, uniqueName('Priv'));
    const holder = await registerAccount(request, 'holder');
    await joinQueueViaApi(request, holder.token, queue.id);
    const outsider = await registerAccount(request, 'outsider');

    const res = await queueStatusViaApi(request, outsider.token, queue.id);
    expect(res.status()).toBe(404);

    await signInWithToken(page, outsider.token, outsider.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-001', { exact: true })).toHaveCount(0);
  });

  test('TC-08-02 Customer tidak bisa memanggil NEXT', async ({ page, request }) => {
    const session = await subscribedOwnerSession(request);
    test.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD.');
    if (!session) return;
    const queue = await createQueueViaApi(request, session.token, uniqueName('NoNext'));
    const customer = await registerAccount(request, 'nonext');
    await joinQueueViaApi(request, customer.token, queue.id);

    const res = await request.post(`${API_URL}/api/v1/queues/${queue.id}/next`, {
      headers: { Authorization: `Bearer ${customer.token}` },
    });
    expect(res.status()).toBe(404);

    await signInWithToken(page, customer.token, customer.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
    await expect(action(page, 'NEXT')).toHaveCount(0);
  });

  test('TC-08-03 Owner lain tidak bisa membuka detail antrian', async ({ page, request }) => {
    const session = await subscribedOwnerSession(request);
    test.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD.');
    if (!session) return;
    const queue = await createQueueViaApi(request, session.token, uniqueName('Owned'));
    const stranger = await registerAccount(request, 'stranger');

    await signInWithToken(page, stranger.token, stranger.email, `/queues/${queue.id}`);

    await expect(page.getByText('Antrian tidak ditemukan')).toBeVisible();
    await expect(page.getByText(queue.queue_name)).toHaveCount(0);
  });

  test('TC-08-04 Token tidak valid dikembalikan ke login', async ({ page }) => {
    await signInWithToken(page, 'token-palsu', 'palsu', '/dashboard');

    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible({ timeout: 15_000 });
  });

  test('TC-08-05 API antrian menolak permintaan tanpa token', async ({ request }) => {
    const res = await request.get(`${API_URL}/api/v1/queues`);
    expect(res.status()).toBe(401);
  });

  test('TC-08-06 Nama antrian yang berisi HTML ditolak', async ({ page, request }) => {
    const session = await subscribedOwnerSession(request);
    test.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD.');
    if (!session) return;
    await signInWithToken(page, session.token, session.displayName);

    await action(page, 'Buat Antrian Baru').click();
    await field(page, 'mis. Loket Pelayanan 1').fill('<b>x</b>');
    await action(page, 'Buat Antrian').click();

    await expect(page.getByText('Data tidak valid, periksa kembali isian Anda')).toBeVisible();
    await expect(page).toHaveURL(/\/dashboard$/);
  });

  test('TC-08-07 Akun tanpa langganan tidak bisa mengubah antrian', async ({ request }) => {
    const session = await subscribedOwnerSession(request);
    test.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD.');
    if (!session) return;
    const queue = await createQueueViaApi(request, session.token, uniqueName('Guard'));
    const nosub = await registerAccount(request, 'guard');

    const res = await request.put(`${API_URL}/api/v1/queues/${queue.id}`, {
      headers: { Authorization: `Bearer ${nosub.token}` },
      data: { queue_name: 'Diubah', queue_description: '' },
    });
    expect([402, 404]).toContain(res.status());
  });
});
