import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  skipViaApi,
  subscribedOwnerSession,
  uniqueName,
} from './support';

async function setup(request: APIRequestContext, customers: number) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Skip'));
  const accounts = [];
  for (let i = 0; i < customers; i++) {
    const account = await registerAccount(request, 'skip');
    await joinQueueViaApi(request, account.token, queue.id);
    accounts.push(account);
  }
  return { session, queue, accounts };
}

test.describe('TS-11 Skip nomor (TC-09-02)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-09-02a Skip tanpa nomor yang dilayani ditolak 409', async ({ request }) => {
    const { session, queue } = await setup(request, 1);
    const res = await skipViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(409);
    expect((await res.json()).error.code).toBe('NOTHING_TO_SKIP');
  });

  test('TC-09-02b Skip tanpa yang menunggu ditolak 409 dan nomor tetap dilayani', async ({ request }) => {
    const { session, queue } = await setup(request, 1);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    const res = await skipViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(409);
    expect((await res.json()).error.code).toBe('QUEUE_EMPTY');
  });

  test('TC-09-02c Skip melewati nomor dan memanggil berikutnya, real-time di customer', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);

    await signInWithToken(page, accounts[0].token, accounts[0].email, `/ticket/${queue.id}`);
    await expect(page.getByText('Sedang dilayani').first()).toBeVisible();

    const res = await skipViaApi(request, session.token, queue.id);
    expect(res.status()).toBe(200);
    expect((await res.json()).data.ticket_number).toBe(2);

    // Lencana status berubah dari "Sedang dilayani" menjadi "Dilewati".
    await expect(page.getByText('Dilewati', { exact: true })).toBeVisible();
  });

  test('TC-09-02d Customer berikutnya melihat Sedang dilayani setelah Skip', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    await signInWithToken(page, accounts[1].token, accounts[1].email, `/ticket/${queue.id}`);
    await expect(page.getByText('Menunggu')).toBeVisible();

    expect((await skipViaApi(request, session.token, queue.id)).status()).toBe(200);

    await expect(page.getByText('Sedang dilayani').first()).toBeVisible();
  });

  test('TC-09-02e Owner menekan tombol Skip, nomor tampil "dilewati"', async ({ page, request }) => {
    const { session, queue } = await setup(request, 2);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    await signInWithToken(page, session.token, session.displayName);
    await expect(page.getByText('A-001', { exact: true }).first()).toBeVisible();

    await action(page, 'Skip').first().click();

    await expect(page.getByText('dilewati').first()).toBeVisible();
  });
});
