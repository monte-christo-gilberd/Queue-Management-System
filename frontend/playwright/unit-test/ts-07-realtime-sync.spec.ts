import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  closeQueueViaApi,
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

async function setup(request: APIRequestContext, customers: number) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('RT'));
  const accounts = [];
  for (let i = 0; i < customers; i++) {
    const account = await registerAccount(request, 'rt');
    await joinQueueViaApi(request, account.token, queue.id);
    accounts.push(account);
  }
  return { session, queue, accounts };
}

test.describe('TS-07 Pembaruan real-time', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-07-01 Dashboard owner menerima customer baru tanpa refresh', async ({ page, request }) => {
    const session = await subscribedOwnerSession(request);
    if (!session) throw new Error('owner berlangganan tidak tersedia');
    const queue = await createQueueViaApi(request, session.token, uniqueName('Live'));
    await signInWithToken(page, session.token, session.displayName);
    await expect(action(page, 'Next').first()).toBeVisible();

    const customer = await registerAccount(request, 'newcomer');
    await joinQueueViaApi(request, customer.token, queue.id);

    // Tanpa page.reload(): event ticket_joined memicu pengambilan ulang daftar tiket.
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
  });

  test('TC-07-02 Status customer diperbarui saat owner menekan NEXT', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    const second = accounts[1];
    await signInWithToken(page, second.token, second.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-002', { exact: true })).toBeVisible();

    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);

    await expect(page.getByText('Sedang dilayani')).toBeVisible();
    await expect(page.getByText('A-001', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Orang di depan Anda')).toBeVisible();
  });

  test('TC-07-03 Customer melihat antrian ditutup tanpa refresh', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 1);
    await signInWithToken(page, accounts[0].token, accounts[0].email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();

    await closeQueueViaApi(request, session.token, queue.id);

    await expect(page.getByText('Antrian ditutup')).toBeVisible();
  });

  test('TC-07-04 Setelah koneksi pulih, status disinkronkan ulang', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    const second = accounts[1];
    await signInWithToken(page, second.token, second.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-002', { exact: true })).toBeVisible();

    // Putuskan stream event: permintaan SSE dibatalkan sampai rute dibuka kembali.
    await page.route('**/event', (route) => route.abort());
    await page.waitForTimeout(500);

    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);

    await page.unroute('**/event');

    // Backoff reconnect maksimal beberapa detik; setelah pulih, status diambil ulang.
    await expect(page.getByText('Sedang dilayani')).toBeVisible({ timeout: 20_000 });
    await expect(page.getByText('A-001', { exact: true }).first()).toBeVisible();
  });

  test('TC-07-05 Event SSE terlewat: status disinkronkan ulang tiap 45 detik', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    const second = accounts[1];
    await page.clock.install();
    // Stream SSE diblokir sejak awal (stream yang sudah terbuka tidak bisa diputus lewat route),
    // sehingga tidak ada event yang sampai, seperti event yang terbuang di hub.
    await page.route('**/event', (route) => route.abort());
    await signInWithToken(page, second.token, second.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-002', { exact: true })).toBeVisible();

    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    // Baris "Sedang dilayani" selalu ada; yang berubah adalah nilainya (A-001 baru ada setelah Next).
    await expect(page.getByText('A-001', { exact: true })).toHaveCount(0);

    await page.clock.fastForward(50_000);

    await expect(page.getByText('A-001', { exact: true }).first()).toBeVisible();
  });

  test('TC-07-06 SSE tidak mengirim apa pun (mis. tunnel): status tetap diperbarui dalam hitungan detik', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    const second = accounts[1];
    await page.route('**/event', (route) => route.abort());
    await signInWithToken(page, second.token, second.email, `/ticket/${queue.id}`);
    await expect(page.getByText('A-002', { exact: true })).toBeVisible();

    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);

    await expect(page.getByText('A-001', { exact: true }).first()).toBeVisible({ timeout: 12_000 });
  });
});
