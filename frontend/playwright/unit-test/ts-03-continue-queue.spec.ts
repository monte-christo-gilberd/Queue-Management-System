import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import {
  action,
  clickNext,
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
  await expect(action(page, 'Next').first()).toBeVisible();
  return queue;
}

/** Penanda "sedang dilayani" tepat di bawah nomor tiket pada timeline. */
function servingMarker(page: import('@playwright/test').Page, n: number) {
  return page
    .getByText(ticketLabel(n), { exact: true })
    .locator('xpath=following-sibling::*[1][normalize-space()="sedang dilayani"]')
    .first();
}

test.describe('TS-03 Melanjutkan antrian (owner)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-03-01 Next pada antrian kosong', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 0);

    await clickNext(page);

    await expect(page.getByText('Tidak ada yang menunggu di antrian ini')).toBeVisible();
    await expect(page.getByText(/sedang dilayani/)).toHaveCount(0);
  });

  test('TC-03-02 Next memajukan nomor yang dilayani', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 2);

    await clickNext(page);

    await expect(servingMarker(page, 1)).toBeVisible();
    await expect(page.getByText(ticketLabel(2), { exact: true }).first()).toBeVisible();
  });

  test('TC-03-03 Klik ganda pada tombol konfirmasi tidak memajukan dua nomor', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 3);

    await action(page, 'Next').first().click();
    await action(page, 'Ya, Lanjut').dblclick();

    await expect(servingMarker(page, 1)).toBeVisible();
    await page.waitForTimeout(1000);
    await expect(servingMarker(page, 2)).toHaveCount(0);
  });

  test('TC-03-04 Next berurutan mengikuti urutan bergabung', async ({ page, request }) => {
    await openOwnerDashboardWithNewQueue(page, request, 3);

    for (const n of [1, 2, 3]) {
      await clickNext(page);
      await expect(servingMarker(page, n)).toBeVisible();
    }
  });
});
