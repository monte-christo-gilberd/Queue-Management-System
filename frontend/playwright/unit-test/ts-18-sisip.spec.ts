import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  field,
  joinQueueViaApi,
  listTicketsViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

/** user_id pemilik tiket waiting, urut sort_key (urutan tunggu sebenarnya), dibaca dari API owner. */
async function waitingUserIds(request: APIRequestContext, ownerToken: string, queueId: number) {
  const res = await listTicketsViaApi(request, ownerToken, queueId);
  expect(res.ok()).toBeTruthy();
  const tickets = (await res.json()).data.tickets as { status: string; user_id: number }[];
  return tickets.filter((t) => t.status === 'waiting').map((t) => t.user_id);
}

/** Antrian baru: customer A dan B sudah menunggu. Owner membuka daftar tiket lalu popup Sisip customer. */
async function openInsertModal(page: Page, request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Sisip'));
  const a = await registerAccount(request, 'cust');
  const b = await registerAccount(request, 'cust');
  const c = await registerAccount(request, 'cust');
  const joinA = (await joinQueueViaApi(request, a.token, queue.id)) as { data: { user_id: number } };
  const joinB = (await joinQueueViaApi(request, b.token, queue.id)) as { data: { user_id: number } };

  await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);
  await action(page, 'sedang dilayani').first().click();
  await expect(action(page, 'Daftar tiket')).toBeVisible();
  await action(page, 'Sisip customer').first().click();
  await expect(field(page, 'contoh: budi@mail.com')).toBeVisible();
  return {
    session,
    queue,
    a,
    c,
    idA: joinA.data.user_id,
    idB: joinB.data.user_id,
  };
}

test.describe('TS-18 Sisip customer di posisi tertentu (US-10)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-18-01 Sisip di posisi 1: akun baru menjadi berikutnya dalam urutan tunggu', async ({ page, request }) => {
    const { session, queue, c, idA, idB } = await openInsertModal(page, request);

    await field(page, 'contoh: budi@mail.com').fill(c.email);
    await field(page, 'contoh: 1').fill('1');
    await action(page, 'Sisip').click();

    await expect(field(page, 'contoh: budi@mail.com')).toHaveCount(0);
    const order = await waitingUserIds(request, session.token, queue.id);
    expect(order.length).toBe(3);
    expect(order.slice(1)).toEqual([idA, idB]);
  });

  test('TC-18-02 Email tidak terdaftar: pesan tampil, popup tetap terbuka', async ({ page, request }) => {
    await openInsertModal(page, request);

    await field(page, 'contoh: budi@mail.com').fill('tidak-ada@contoh.test');
    await action(page, 'Sisip').click();

    await expect(action(page, 'Akun tidak ditemukan. Periksa email atau username.')).toBeVisible();
    await expect(field(page, 'contoh: budi@mail.com')).toBeVisible();
  });

  test('TC-18-03 Akun yang sudah ikut antrian: pesan tampil, tidak ada tiket ganda', async ({ page, request }) => {
    const { session, queue, a } = await openInsertModal(page, request);

    await field(page, 'contoh: budi@mail.com').fill(a.email);
    await action(page, 'Sisip').click();

    await expect(action(page, 'Akun ini sudah ikut antrian.')).toBeVisible();
    expect((await waitingUserIds(request, session.token, queue.id)).length).toBe(2);
  });
});
