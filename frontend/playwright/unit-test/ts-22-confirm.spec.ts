import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  listTicketsViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

/** Status tiket di antrian dari API owner (sumber kebenaran). */
async function statusOf(request: APIRequestContext, token: string, queueId: number, ticketNumber: number) {
  const res = await listTicketsViaApi(request, token, queueId);
  expect(res.ok()).toBeTruthy();
  const rows = (await res.json()).data.tickets as { ticket_number: number; status: string }[];
  return rows.find((t) => t.ticket_number === ticketNumber)?.status;
}

/** Antrian dengan dua customer; nomor 1 sudah dilayani. */
async function setup(request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Confirm'));
  for (let i = 0; i < 2; i++) {
    const c = await registerAccount(request, 'conf');
    await joinQueueViaApi(request, c.token, queue.id);
  }
  expect((await nextViaApi(request, session.token, queue.id)).status()).toBe(200);
  return { session, queue };
}

test.describe('TS-22 Popup konfirmasi Next dan Skip (4.21)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-22-01 Next menampilkan popup yang menyebut nomor, dan Batal tidak mengubah apa pun', async ({ page, request }) => {
    const { session, queue } = await setup(request);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);

    await action(page, 'NEXT').click();
    await expect(action(page, 'Lanjut ke nomor berikutnya?')).toBeVisible();
    await expect(page.getByText('A-001 akan ditandai selesai dilayani')).toBeVisible();
    await action(page, 'Batal').click();
    await expect(action(page, 'Lanjut ke nomor berikutnya?')).toBeHidden();

    expect(await statusOf(request, session.token, queue.id, 1)).toBe('currently_served');
    expect(await statusOf(request, session.token, queue.id, 2)).toBe('waiting');
  });

  test('TC-22-02 Konfirmasi Next memajukan antrian', async ({ page, request }) => {
    const { session, queue } = await setup(request);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);

    await action(page, 'NEXT').click();
    await action(page, 'Ya, Lanjut').click();

    await expect.poll(() => statusOf(request, session.token, queue.id, 1)).toBe('completed');
    expect(await statusOf(request, session.token, queue.id, 2)).toBe('currently_served');
  });

  test('TC-22-03 Skip menampilkan popup, Batal tidak mengubah, konfirmasi melewati nomor', async ({ page, request }) => {
    const { session, queue } = await setup(request);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);

    await action(page, 'SKIP').click();
    await expect(action(page, 'Lewati A-001?')).toBeVisible();
    await action(page, 'Batal').click();
    expect(await statusOf(request, session.token, queue.id, 1)).toBe('currently_served');

    await action(page, 'SKIP').click();
    await action(page, 'Ya, Lewati').click();
    await expect.poll(() => statusOf(request, session.token, queue.id, 1)).toBe('skipped');
    expect(await statusOf(request, session.token, queue.id, 2)).toBe('currently_served');
  });

  test('TC-22-04 Layar live meminta konfirmasi sebelum Next', async ({ page, request }) => {
    const { session, queue } = await setup(request);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}/live`);

    await action(page, 'Next').first().click();
    await expect(action(page, 'Lanjut ke nomor berikutnya?')).toBeVisible();
    await action(page, 'Batal').click();

    expect(await statusOf(request, session.token, queue.id, 1)).toBe('currently_served');
  });

  test('TC-22-05 Popup di dashboard meminta konfirmasi sebelum Skip', async ({ page, request }) => {
    const { session, queue } = await setup(request);
    await signInWithToken(page, session.token, session.displayName);

    await action(page, 'Skip').first().click();
    await expect(action(page, 'Lewati A-001?')).toBeVisible();
    await action(page, 'Batal').click();

    expect(await statusOf(request, session.token, queue.id, 1)).toBe('currently_served');
  });
});
