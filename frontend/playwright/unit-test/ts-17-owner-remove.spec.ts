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

type TicketRow = { id: number; ticket_number: number; status: string; called_counter_id?: number | null };
type CounterRow = { id: number; is_default: boolean };

function auth(token: string) {
  return { Authorization: `Bearer ${token}` };
}

async function listTickets(request: APIRequestContext, ownerToken: string, queueId: number): Promise<TicketRow[]> {
  const res = await listTicketsViaApi(request, ownerToken, queueId);
  expect(res.ok()).toBeTruthy();
  return (await res.json()).data.tickets as TicketRow[];
}

async function ticketByNumber(request: APIRequestContext, ownerToken: string, queueId: number, ticketNumber: number) {
  const found = (await listTickets(request, ownerToken, queueId)).find((t) => t.ticket_number === ticketNumber);
  if (!found) throw new Error(`tiket ${ticketNumber} tidak ada`);
  return found;
}

async function removeOn(request: APIRequestContext, token: string, queueId: number, ticketId: number) {
  return request.post(`${API_URL}/api/v1/queues/${queueId}/tickets/${ticketId}/remove`, { headers: auth(token) });
}

async function listCounters(request: APIRequestContext, token: string, queueId: number): Promise<CounterRow[]> {
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/counters`, { headers: auth(token) });
  expect(res.status()).toBe(200);
  return (await res.json()).data.counters as CounterRow[];
}

async function createCounter(request: APIRequestContext, token: string, queueId: number): Promise<CounterRow> {
  const res = await request.post(`${API_URL}/api/v1/queues/${queueId}/counters`, { headers: auth(token), data: {} });
  expect(res.status()).toBe(201);
  return (await res.json()).data as CounterRow;
}

async function nextOn(request: APIRequestContext, token: string, queueId: number, counterId: number) {
  return request.post(`${API_URL}/api/v1/queues/${queueId}/next`, {
    headers: auth(token),
    data: { counter_id: counterId },
  });
}

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

  test('TC-17-04 Hapus nomor yang sedang dilayani Counter 2: Counter 2 mendapat nomor berikutnya, Counter 1 tidak berubah', async ({ request }) => {
    const { owner, queue, a, b } = await setupQueue(request);
    const c = await registerAccount(request, 'cust');
    await joinQueueViaApi(request, c.token, queue.id);
    const counters = await listCounters(request, owner.token, queue.id);
    const first = counters.find((x) => x.is_default);
    if (!first) throw new Error('counter default tidak ada');
    const second = await createCounter(request, owner.token, queue.id);

    // Counter 1 melayani nomor 1, Counter 2 melayani nomor 2, nomor 3 menunggu.
    expect((await nextOn(request, owner.token, queue.id, first.id)).status()).toBe(200);
    expect((await nextOn(request, owner.token, queue.id, second.id)).status()).toBe(200);
    expect(a.token && b.token).toBeTruthy();

    const target = await ticketByNumber(request, owner.token, queue.id, 2);
    expect((await removeOn(request, owner.token, queue.id, target.id)).status()).toBe(200);

    const rows = await listTickets(request, owner.token, queue.id);
    const byNumber = Object.fromEntries(rows.map((t) => [t.ticket_number, t]));
    expect(byNumber[2].status).toBe('deleted');
    expect(byNumber[3].status).toBe('currently_served');
    expect(byNumber[3].called_counter_id).toBe(second.id);
    expect(byNumber[1].status).toBe('currently_served');
    expect(byNumber[1].called_counter_id).toBe(first.id);
  });

  test('TC-17-05 Hapus nomor yang sama dua kali tidak error dan status tetap Dihapus', async ({ request }) => {
    const { owner, queue } = await setupQueue(request);
    const target = await ticketByNumber(request, owner.token, queue.id, 2);

    expect((await removeOn(request, owner.token, queue.id, target.id)).status()).toBe(200);
    const again = await removeOn(request, owner.token, queue.id, target.id);
    expect(again.status()).toBe(200);
    expect((await again.json()).data.status).toBe('deleted');
  });

  test('TC-17-06 Bukan owner tidak bisa menghapus nomor (404)', async ({ request }) => {
    const { owner, queue, a } = await setupQueue(request);
    const target = await ticketByNumber(request, owner.token, queue.id, 2);

    const res = await removeOn(request, a.token, queue.id, target.id);
    expect(res.status()).toBe(404);
    expect(await ticketStatus(request, owner.token, queue.id, 2)).toBe('waiting');
  });

  test('TC-17-07 Customer yang nomornya dihapus melihat Dihapus langsung tanpa muat ulang', async ({ page, request }) => {
    const { owner, queue, b } = await setupQueue(request);
    await signInWithToken(page, b.token, b.username, `/ticket/${queue.id}`);
    await expect(action(page, 'Menunggu')).toBeVisible();

    const target = await ticketByNumber(request, owner.token, queue.id, 2);
    expect((await removeOn(request, owner.token, queue.id, target.id)).status()).toBe(200);

    await expect(action(page, 'Dihapus')).toBeVisible();
  });
});
