import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import {
  action,
  API_URL,
  createQueueViaApi,
  joinQueueViaApi,
  queueStatusViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

/** Antrian baru dengan satu customer yang sudah bergabung; customer membuka halaman status miliknya. */
async function openCustomerStatus(page: Page, request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Cancel'));
  const customer = await registerAccount(request, 'cust');
  await joinQueueViaApi(request, customer.token, queue.id);

  await signInWithToken(page, customer.token, customer.username, `/ticket/${queue.id}`);
  await expect(action(page, 'Batalkan nomor saya')).toBeVisible();
  return { queue, customer };
}

async function myTicketStatus(request: APIRequestContext, token: string, queueId: number) {
  const res = await queueStatusViaApi(request, token, queueId);
  expect(res.ok()).toBeTruthy();
  return (await res.json()).data.my_ticket as { status: string; ticket_number: number } | null;
}

test.describe('TS-16 Customer membatalkan nomornya (4.9)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-16-01 Batalkan nomor menunggu: status Dibatalkan, tombol batal hilang', async ({ page, request }) => {
    const { queue, customer } = await openCustomerStatus(page, request);

    await action(page, 'Batalkan nomor saya').click();
    await expect(action(page, 'Batalkan nomor ini?')).toBeVisible();
    await action(page, 'Ya, Batalkan').click();

    await expect(action(page, 'Dibatalkan')).toBeVisible();
    await expect(action(page, 'Batalkan nomor saya')).toHaveCount(0);
    const ticket = await myTicketStatus(request, customer.token, queue.id);
    expect(ticket?.status).toBe('cancelled');
  });

  test('TC-16-02 Batal lalu bergabung lagi: nomor baru, nomor lama tetap Dibatalkan', async ({ page, request }) => {
    const { queue, customer } = await openCustomerStatus(page, request);
    await action(page, 'Batalkan nomor saya').click();
    await action(page, 'Ya, Batalkan').click();
    await expect(action(page, 'Dibatalkan')).toBeVisible();

    const rejoined = (await joinQueueViaApi(request, customer.token, queue.id)) as {
      data: { ticket_number: number; status: string };
    };

    expect(rejoined.data.ticket_number).toBe(2);
    expect(rejoined.data.status).toBe('waiting');
  });

  test('TC-16-03 Batal dibatalkan di dialog: nomor tetap aktif', async ({ page, request }) => {
    const { queue, customer } = await openCustomerStatus(page, request);

    await action(page, 'Batalkan nomor saya').click();
    await action(page, 'Tidak').click();

    await expect(action(page, 'Batalkan nomor saya')).toBeVisible();
    const ticket = await myTicketStatus(request, customer.token, queue.id);
    expect(ticket?.status).toBe('waiting');
  });

  test('TC-16-04 Batal nomor yang sedang dilayani: nomor berikutnya naik ke Sedang dilayani', async ({ page, request }) => {
    const { queue, customer } = await openCustomerStatus(page, request);
    const next = await registerAccount(request, 'cust');
    await joinQueueViaApi(request, next.token, queue.id);

    const owner = await subscribedOwnerSession(request);
    const nextRes = await request.post(`${API_URL}/api/v1/queues/${queue.id}/next`, {
      headers: { Authorization: `Bearer ${owner!.token}` },
    });
    expect(nextRes.ok()).toBeTruthy();

    await page.reload();
    await action(page, 'Batalkan nomor saya').click();
    await action(page, 'Ya, Batalkan').click();

    await expect(action(page, 'Dibatalkan')).toBeVisible();
    const after = await myTicketStatus(request, next.token, queue.id);
    expect(after?.status).toBe('currently_served');
    expect(after?.ticket_number).toBe(2);
  });
});
