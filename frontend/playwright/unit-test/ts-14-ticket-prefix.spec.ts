import { expect, test } from '@playwright/test';

import {
  action,
  closeQueueViaApi,
  createQueueViaApi,
  field,
  joinQueueViaApi,
  openCreateQueue,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  WEB_URL,
} from './support';
import { API_URL } from './support/env';

test.describe('TS-14 Awalan nomor tiket', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  async function owner(request: import('@playwright/test').APIRequestContext) {
    const session = await subscribedOwnerSession(request);
    if (!session) throw new Error('owner berlangganan tidak tersedia');
    return session;
  }

  test('TC-14-01 Membuat antrian dengan awalan dari form', async ({ page, request }) => {
    const session = await owner(request);
    await signInWithToken(page, session.token, session.displayName);

    await openCreateQueue(page);
    await field(page, 'mis. Loket Pelayanan 1').fill(uniqueName('Pref'));
    await field(page, 'Opsional, 1-3 huruf/angka (default A)').fill('vip');
    await action(page, 'Buat Antrian').click();

    await expect(page).toHaveURL(/\/queues\/\d+$/);
    await expect(page.locator('input[value="VIP"]')).toBeVisible();
  });

  test('TC-14-02 Tanpa awalan, nomor memakai A', async ({ request }) => {
    const session = await owner(request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Def'));
    expect(queue.ticket_prefix).toBe('A');
  });

  test('TC-14-03 API menolak awalan tidak valid', async ({ request }) => {
    const session = await owner(request);
    for (const bad of ['ABCD', 'A-', 'a b', '<b>']) {
      const res = await request.post(`${API_URL}/api/v1/queues`, {
        headers: { Authorization: `Bearer ${session.token}` },
        data: { queue_name: uniqueName('Bad'), queue_description: '', ticket_prefix: bad },
      });
      expect(res.status(), bad).toBe(400);
    }
  });

  test('TC-14-04 Awalan tampil di nomor customer dan bisa diubah owner', async ({ page, request }) => {
    const session = await owner(request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Tik'));
    const customer = await registerAccount(request, 'pf');
    await joinQueueViaApi(request, customer.token, queue.id);

    const res = await request.put(`${API_URL}/api/v1/queues/${queue.id}`, {
      headers: { Authorization: `Bearer ${session.token}` },
      data: { queue_name: queue.queue_name, queue_description: '', ticket_prefix: 'b2' },
    });
    expect(res.status(), await res.text()).toBe(200);

    await signInWithToken(page, customer.token, customer.email, `/ticket/${queue.id}`);
    await expect(page.getByText('B2-001', { exact: true })).toBeVisible();
    await closeQueueViaApi(request, session.token, queue.id);
  });

  test('TC-14-05 Antrian lama tetap A-001 dan tidak berubah', async ({ page, request }) => {
    const session = await owner(request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Lama'));
    const customer = await registerAccount(request, 'pf');
    await joinQueueViaApi(request, customer.token, queue.id);

    await signInWithToken(page, customer.token, customer.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
    await page.goto(`${WEB_URL}/dashboard`);
    await closeQueueViaApi(request, session.token, queue.id);
  });
});
