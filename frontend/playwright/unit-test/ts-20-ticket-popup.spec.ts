import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  ticketLabel,
  uniqueName,
} from './support';

const HINT = 'ketuk untuk melihat tiket Anda ▾';

async function setup(request: APIRequestContext, customers: number) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Popup'));
  const accounts = [];
  for (let i = 0; i < customers; i++) {
    const account = await registerAccount(request, 'popup');
    await joinQueueViaApi(request, account.token, queue.id);
    accounts.push(account);
  }
  return { session, queue, accounts };
}

test.describe('TS-20 Popup tiket di kartu antrian customer', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-20-01 Ketuk kartu menampilkan tiket customer yang menunggu', async ({ page, request }) => {
    const { accounts } = await setup(request, 2);
    await signInWithToken(page, accounts[1].token, accounts[1].email);

    await action(page, HINT).click();
    await expect(action(page, 'Tiket Anda')).toBeVisible();
    await expect(action(page, ticketLabel(2))).toBeVisible();
    await expect(action(page, 'Menunggu')).toBeVisible();
    await expect(action(page, 'Di depan Anda')).toBeVisible();
    await expect(action(page, '1 orang')).toBeVisible();
  });

  test('TC-20-02 Popup bisa ditutup dengan tombol Tutup', async ({ page, request }) => {
    const { accounts } = await setup(request, 1);
    await signInWithToken(page, accounts[0].token, accounts[0].email);

    await action(page, HINT).click();
    await expect(action(page, 'Tiket Anda')).toBeVisible();
    await action(page, 'Tutup').click();
    await expect(action(page, 'Tiket Anda')).toBeHidden();
  });

  test('TC-20-03 Setelah dipanggil, popup tidak lagi menampilkan jumlah orang di depan', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 1);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    await signInWithToken(page, accounts[0].token, accounts[0].email);

    await action(page, HINT).click();
    // Nomor sendiri muncul beberapa kali (kartu, nomor besar, baris "Sedang dilayani"), jadi cek popup lewat judulnya.
    await expect(action(page, 'Tiket Anda')).toBeVisible();
    await expect(action(page, 'Di depan Anda')).toBeHidden();
  });
});
