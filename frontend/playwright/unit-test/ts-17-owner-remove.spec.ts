import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import {
  action,
  API_URL,
  createQueueViaApi,
  joinQueueViaApi,
  listTicketsViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

/** Status tiket di antrian, dibaca dari API owner (sumber kebenaran). */
async function ticketStatus(request: APIRequestContext, ownerToken: string, queueId: number, ticketNumber: number) {
  const res = await listTicketsViaApi(request, ownerToken, queueId);
  expect(res.ok()).toBeTruthy();
  const tickets = (await res.json()).data.tickets as { ticket_number: number; status: string }[];
  return tickets.find((t) => t.ticket_number === ticketNumber)?.status;
}

/** Antrian baru: customer A (nomor 1) dan B (nomor 2) sudah bergabung. */
async function setupQueue(request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Remove'));
  const a = await registerAccount(request, 'cust');
  const b = await registerAccount(request, 'cust');
  await joinQueueViaApi(request, a.token, queue.id);
  await joinQueueViaApi(request, b.token, queue.id);
  return { owner: session, queue, a, b };
}

/** Buka halaman detail antrian lalu daftar tiket (kartu customer). */
async function openTicketList(page: Page, ownerToken: string, displayName: string, queueId: number) {
  await signInWithToken(page, ownerToken, displayName, `/queues/${queueId}`);
  await action(page, 'sedang dilayani').first().click();
  await expect(action(page, 'Daftar tiket')).toBeVisible();
}

test.describe('TS-17 Owner menghapus nomor customer (4.10)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-17-01 Hapus nomor menunggu: status Dihapus, nomor tidak kembali ke urutan tunggu', async ({ page, request }) => {
    const { owner, queue, b } = await setupQueue(request);
    await openTicketList(page, owner.token, owner.displayName, queue.id);

    await action(page, 'Hapus').last().click();
    await expect(action(page, 'Ya, Hapus')).toBeVisible();
    await action(page, 'Ya, Hapus').click();

    await expect(action(page, 'Ya, Hapus')).toHaveCount(0);
    expect(await ticketStatus(request, owner.token, queue.id, 2)).toBe('deleted');

    await signInWithToken(page, b.token, b.username, `/ticket/${queue.id}`);
    await expect(action(page, 'Dihapus')).toBeVisible();
  });

  test('TC-17-02 Hapus nomor yang sedang dilayani: nomor berikutnya naik', async ({ page, request }) => {
    const { owner, queue } = await setupQueue(request);
    await nextViaApi(request, owner.token, queue.id);
    await openTicketList(page, owner.token, owner.displayName, queue.id);

    await action(page, 'Hapus').first().click();
    await action(page, 'Ya, Hapus').click();

    await expect(action(page, 'Ya, Hapus')).toHaveCount(0);
    expect(await ticketStatus(request, owner.token, queue.id, 1)).toBe('deleted');
    expect(await ticketStatus(request, owner.token, queue.id, 2)).toBe('currently_served');
  });

  test('TC-17-03 Batal di dialog: nomor tetap aktif', async ({ page, request }) => {
    const { owner, queue } = await setupQueue(request);
    await openTicketList(page, owner.token, owner.displayName, queue.id);

    await action(page, 'Hapus').last().click();
    await action(page, 'Batal').click();

    await expect(action(page, 'Ya, Hapus')).toHaveCount(0);
    expect(await ticketStatus(request, owner.token, queue.id, 2)).toBe('waiting');
  });
});
