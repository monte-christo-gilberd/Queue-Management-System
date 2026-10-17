import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  ticketLabel,
  uniqueName,
  WEB_URL,
} from './support';

async function openOwnerDashboardWithNewQueue(page: Page, request: APIRequestContext, customers: number) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');

  // Antrian baru selalu menjadi antrian aktif teratas di dashboard.
  const queue = await createQueueViaApi(request, session.token, uniqueName('Next'));
  for (let i = 0; i < customers; i++) {
    const customer = await registerAccount(request, 'cust');
    await joinQueueViaApi(request, customer.token, queue.id);
  }

  await signInWithToken(page, session.token, session.displayName);
  // Dashboard hanya menampilkan papan tiket (dengan tombol NEXT) ketika ada antrian aktif.
  await expect(action(page, 'NEXT')).toBeVisible();
  return queue;
}

test.describe('TS-03 Melanjutkan antrian (owner)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-03-01 Next pada antrian kosong', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 0);

    await action(page, 'NEXT').click();

    await expect(page.getByText('Tidak ada yang menunggu di antrian ini')).toBeVisible();
    await expect(page.getByText(/sedang dilayani/)).toHaveCount(0);
  });

  test('TC-03-02 Next memajukan nomor yang dilayani', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 2);

    await action(page, 'NEXT').click();

    await expect(page.getByText(`Nomor ${ticketLabel(1)} sedang dilayani`)).toBeVisible();
    await expect(page.getByText(ticketLabel(2), { exact: true }).first()).toBeVisible();
  });

  test('TC-03-03 Klik ganda tidak memajukan dua nomor', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 3);

    await action(page, 'NEXT').dblclick();

    await expect(page.getByText(`Nomor ${ticketLabel(1)} sedang dilayani`)).toBeVisible();
    await page.waitForTimeout(1000);
    await expect(page.getByText(`Nomor ${ticketLabel(2)} sedang dilayani`)).toHaveCount(0);
  });

  test('TC-03-04 Next berurutan mengikuti urutan bergabung', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 3);

    for (const n of [1, 2, 3]) {
      await action(page, 'NEXT').click();
      await expect(page.getByText(`Nomor ${ticketLabel(n)} sedang dilayani`)).toBeVisible();
    }
  });
});
