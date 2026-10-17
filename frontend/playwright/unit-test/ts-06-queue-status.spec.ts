import { expect, test, type APIRequestContext, type Browser } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  registerAccount,
  signInThroughUi,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  WEB_URL,
} from './support';

async function queueWithCustomers(request: APIRequestContext, count: number) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Status'));
  const customers = [];
  for (let i = 0; i < count; i++) {
    const customer = await registerAccount(request, 'st');
    await joinQueueViaApi(request, customer.token, queue.id);
    customers.push(customer);
  }
  return { queue, ownerToken: session.token, customers };
}

test.describe('TS-06 Status antrian (customer)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-06-01 Status menampilkan nomor, nomor dilayani, dan jumlah di depan', async ({ page, request }) => {
    const { queue, customers } = await queueWithCustomers(request, 3);
    const third = customers[2];

    await signInWithToken(page, third.token, third.email, `/join/${queue.id}`);

    await expect(page.getByText('A-003', { exact: true })).toBeVisible();
    await expect(page.getByText('Orang di depan Anda')).toBeVisible();
    await expect(page.getByText('2', { exact: true })).toBeVisible();
    await expect(page.getByText('Sedang dilayani')).toBeVisible();
  });

  test('TC-06-02 Setelah dilayani, status berubah menjadi Sudah dilayani', async ({ page, request }) => {
    const { queue, ownerToken, customers } = await queueWithCustomers(request, 2);
    expect((await nextViaApi(request, ownerToken, queue.id)).status()).toBe(200);

    await signInWithToken(page, customers[0].token, customers[0].email, `/join/${queue.id}`);

    await expect(page.getByText('Sudah dilayani')).toBeVisible();
    await expect(page.getByText('A-001', { exact: true }).first()).toBeVisible();
  });

  test('TC-06-03 Status sama di perangkat lain', async ({ browser, request }: { browser: Browser; request: APIRequestContext }) => {
    const { queue, customers } = await queueWithCustomers(request, 1);
    const customer = customers[0];

    // Perangkat lain: konteks browser baru tanpa penyimpanan sesi, login manual.
    const context = await browser.newContext();
    const otherDevice = await context.newPage();
    try {
      await signInThroughUi(otherDevice, customer.email, customer.password);
      await otherDevice.goto(`${WEB_URL}/join/${queue.id}`);

      await expect(otherDevice.getByText('A-001', { exact: true })).toBeVisible();
      await expect(action(otherDevice, 'Gabung Antrian')).toHaveCount(0);
    } finally {
      await context.close();
    }
  });

  test('TC-06-04 Status tetap tampil setelah dibuka ulang', async ({ page, request }) => {
    const { queue, customers } = await queueWithCustomers(request, 1);
    const customer = customers[0];
    await signInWithToken(page, customer.token, customer.email, `/join/${queue.id}`);
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();

    await page.reload();

    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
    await expect(action(page, 'Gabung Antrian')).toHaveCount(0);
  });
});
