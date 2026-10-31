import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  setSkipSettingsViaApi,
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

test.describe('TS-12 Pengaturan Skip (fallback)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('pengaturan awal remove, N null, batas 1', async ({ request }) => {
    const { session, queue } = await setup(request, 1);
    const res = await request.get(`${process.env.QMS_API_URL ?? 'http://localhost:8080'}/api/v1/queues/${queue.id}/settings/skip`, {
      headers: { Authorization: `Bearer ${session.token}` },
    });
    expect(res.status()).toBe(200);
    expect((await res.json()).data).toEqual({ fallback: 'remove', n: null, max_returns: 1 });
  });

  test('pengaturan tidak valid ditolak 400', async ({ request }) => {
    const { session, queue } = await setup(request, 1);
    for (const body of [
      { fallback: 'after_n' as const, n: 0, max_returns: 1 },
      { fallback: 'back' as const, max_returns: 0 },
      { fallback: 'zzz' as never, max_returns: 1 },
    ]) {
      const res = await setSkipSettingsViaApi(request, session.token, queue.id, body);
      expect(res.status()).toBe(400);
    }
  });

  test('customer tidak boleh mengubah pengaturan', async ({ request }) => {
    const { queue, accounts } = await setup(request, 1);
    const res = await setSkipSettingsViaApi(request, accounts[0].token, queue.id, { fallback: 'back', max_returns: 1 });
    expect([402, 403, 404]).toContain(res.status());
  });

  test('mode back: nomor yang dilewati kembali Menunggu di paling belakang', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 3);
    expect((await setSkipSettingsViaApi(request, session.token, queue.id, { fallback: 'back', max_returns: 1 })).status()).toBe(200);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    await signInWithToken(page, accounts[0].token, accounts[0].email, `/ticket/${queue.id}`);
    await expect(page.getByText('Sedang dilayani').first()).toBeVisible();

    expect((await skipViaApi(request, session.token, queue.id)).status()).toBe(200);

    await expect(page.getByText('Menunggu').first()).toBeVisible();
    await expect(page.getByText('Dilewati', { exact: true })).toHaveCount(0);
  });

  test('mode remove tetap menandai Dilewati', async ({ page, request }) => {
    const { session, queue, accounts } = await setup(request, 2);
    expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
    await signInWithToken(page, accounts[0].token, accounts[0].email, `/ticket/${queue.id}`);
    expect((await skipViaApi(request, session.token, queue.id)).status()).toBe(200);
    await expect(page.getByText('Dilewati', { exact: true })).toBeVisible();
  });

  test('batas pengembalian: setelah dikembalikan sekali, skip kedua permanen', async ({ request }) => {
    const { session, queue } = await setup(request, 3);
    await setSkipSettingsViaApi(request, session.token, queue.id, { fallback: 'back', max_returns: 1 });
    await nextViaApi(request, session.token, queue.id); // 1 dilayani
    await skipViaApi(request, session.token, queue.id); // 1 kembali, 2 dilayani
    await skipViaApi(request, session.token, queue.id); // 2 kembali, 3 dilayani
    await nextViaApi(request, session.token, queue.id); // 1 dilayani lagi
    const res = await skipViaApi(request, session.token, queue.id); // 1 permanen, 2 dilayani
    expect(res.status()).toBe(200);
    const list = await (
      await request.get(`${process.env.QMS_API_URL ?? 'http://localhost:8080'}/api/v1/queues/${queue.id}/tickets?status=skipped`, {
        headers: { Authorization: `Bearer ${session.token}` },
      })
    ).json();
    const skipped = JSON.stringify(list);
    expect(skipped).toContain('"ticket_number":1');
  });

  test('UI: owner memilih Mundur N, menyimpan, dan tersimpan setelah reload', async ({ page, request }) => {
    const { session, queue } = await setup(request, 1);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);
    const card = page.getByTestId('skip-settings');
    await expect(card).toBeVisible();

    await card.getByRole('radio', { name: /Mundur N nomor/ }).click();
    await card.getByRole('textbox').first().fill('0');
    await card.getByText('SIMPAN PENGATURAN').click();
    await expect(card.getByText('Jumlah nomor harus angka 1 atau lebih')).toBeVisible();

    await card.getByRole('textbox').first().fill('2');
    await card.getByText('SIMPAN PENGATURAN').click();
    await expect(card.getByText('Pengaturan tersimpan')).toBeVisible();

    await page.reload();
    const again = page.getByTestId('skip-settings');
    await expect(again.getByRole('textbox').first()).toHaveValue('2');
  });
});
