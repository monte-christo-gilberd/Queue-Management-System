import { expect, test, type APIRequestContext } from '@playwright/test';

import { action, clickNext, createQueueViaApi, signInWithToken, subscribedOwnerSession, uniqueName } from './support';

const EMPTY_QUEUE_ERROR = 'Tidak ada yang menunggu di antrian ini';

async function openEmptyQueue(request: APIRequestContext, page: import('@playwright/test').Page) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Toast'));
  await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);
  await expect(action(page, 'NEXT')).toBeVisible();
}

test.describe('TS-21 Toast galat', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-21-01 Galat tampil sebagai toast dan bisa ditutup dengan tombol ×', async ({ page, request }) => {
    await openEmptyQueue(request, page);

    await clickNext(page, 'NEXT');
    const toast = page.getByRole('alert').filter({ hasText: EMPTY_QUEUE_ERROR });
    await expect(toast).toBeVisible();

    await toast.getByRole('button', { name: 'Tutup pesan' }).click();
    await expect(toast).toBeHidden();
  });

  test('TC-21-02 Toast hilang sendiri setelah sekitar 5 detik', async ({ page, request }) => {
    await openEmptyQueue(request, page);

    await clickNext(page, 'NEXT');
    const toast = page.getByRole('alert').filter({ hasText: EMPTY_QUEUE_ERROR });
    await expect(toast).toBeVisible();
    // Masih tampil sebelum 5 detik, lalu hilang tanpa diklik.
    await page.waitForTimeout(3000);
    await expect(toast).toBeVisible();
    await expect(toast).toBeHidden({ timeout: 5000 });
  });

  test('TC-21-03 Beberapa galat menumpuk dan tidak saling menutupi', async ({ page, request }) => {
    await openEmptyQueue(request, page);

    // Galat pertama: validasi pengaturan Skip. Galat kedua: Next di antrian kosong.
    const card = page.getByTestId('skip-settings');
    await card.getByRole('radio', { name: /Mundur N nomor/ }).click();
    await card.getByRole('textbox').first().fill('0');
    await card.getByText('SIMPAN PENGATURAN').click();
    await clickNext(page, 'NEXT');

    const first = page.getByRole('alert').filter({ hasText: 'Jumlah nomor harus angka 1 atau lebih' });
    const second = page.getByRole('alert').filter({ hasText: EMPTY_QUEUE_ERROR });
    await expect(first).toBeVisible();
    await expect(second).toBeVisible();

    // Toast baru di bawah toast lama: yang lama bergeser ke atas.
    const a = await first.boundingBox();
    const b = await second.boundingBox();
    expect(a && b && a.y < b.y).toBe(true);
  });

  test('TC-21-04 Galat yang sama tidak ditumpuk dua kali', async ({ page, request }) => {
    await openEmptyQueue(request, page);

    await clickNext(page, 'NEXT');
    await expect(page.getByRole('alert').filter({ hasText: EMPTY_QUEUE_ERROR })).toHaveCount(1);
    await clickNext(page, 'NEXT');
    await expect(page.getByRole('alert').filter({ hasText: EMPTY_QUEUE_ERROR })).toHaveCount(1);
  });
});
