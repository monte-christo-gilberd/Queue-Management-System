import { expect, test, type APIRequestContext, type Page } from '@playwright/test';

import {
  action,
  API_URL,
  createQueueViaApi,
  field,
  joinQueueViaApi,
  listTicketsViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

const NOTE_PLACEHOLDER = 'Tulis catatan untuk nomor ini...';
const NOTE_ROW_HINT = 'Catatan untuk nomor saat ini';

/** Catatan nomor yang sedang dilayani, dibaca langsung dari API sebagai sumber kebenaran. */
async function noteViaApi(request: APIRequestContext, token: string, queueId: number, ticketNumber: number) {
  const tickets = await (await listTicketsViaApi(request, token, queueId)).json();
  const ticket = tickets.data.tickets.find((t: { ticket_number: number }) => t.ticket_number === ticketNumber);
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/tickets/${ticket.id}/note`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).data.note as string | null;
}

/** Antrian baru dengan satu customer yang sudah dipanggil (nomor 1 sedang dilayani). */
async function openDashboardWithServingNumber(page: Page, request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Notes'));
  const customer = await registerAccount(request, 'cust');
  await joinQueueViaApi(request, customer.token, queue.id);
  await nextViaApi(request, session.token, queue.id);

  await signInWithToken(page, session.token, session.displayName);
  await expect(action(page, NOTE_ROW_HINT).first()).toBeVisible();
  return { session, queue };
}

async function openNoteModal(page: Page) {
  await action(page, NOTE_ROW_HINT).first().click();
  await expect(field(page, NOTE_PLACEHOLDER)).toBeVisible();
}

test.describe('TS-15 Catatan nomor yang sedang dilayani (owner)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-15-01 Simpan pertama membuat catatan, dan API membacanya', async ({ page, request }) => {
    const { session, queue } = await openDashboardWithServingNumber(page, request);

    await openNoteModal(page);
    await expect(field(page, NOTE_PLACEHOLDER)).toHaveValue('');
    await field(page, NOTE_PLACEHOLDER).fill('pelanggan minta dipanggil ulang');
    await action(page, 'Append Note').last().click();

    await expect(field(page, NOTE_PLACEHOLDER)).toHaveCount(0);
    expect(await noteViaApi(request, session.token, queue.id, 1)).toBe('pelanggan minta dipanggil ulang');
  });

  test('TC-15-02 Buka lagi: catatan lama terisi, simpan menimpa di tempat', async ({ page, request }) => {
    const { session, queue } = await openDashboardWithServingNumber(page, request);
    await openNoteModal(page);
    await field(page, NOTE_PLACEHOLDER).fill('versi pertama');
    await action(page, 'Append Note').last().click();
    await expect(field(page, NOTE_PLACEHOLDER)).toHaveCount(0);

    await openNoteModal(page);
    await expect(field(page, NOTE_PLACEHOLDER)).toHaveValue('versi pertama', { timeout: 10_000 });
    await field(page, NOTE_PLACEHOLDER).fill('versi kedua');
    await page.getByText('Simpan Catatan', { exact: true }).filter({ visible: true }).click();

    await expect(field(page, NOTE_PLACEHOLDER)).toHaveCount(0);
    expect(await noteViaApi(request, session.token, queue.id, 1)).toBe('versi kedua');
  });

  test('TC-15-03 Catatan kosong ditolak: popup tetap terbuka, catatan lama utuh', async ({ page, request }) => {
    const { session, queue } = await openDashboardWithServingNumber(page, request);
    await openNoteModal(page);
    await field(page, NOTE_PLACEHOLDER).fill('asli');
    await action(page, 'Append Note').last().click();
    await expect(field(page, NOTE_PLACEHOLDER)).toHaveCount(0);

    await openNoteModal(page);
    await field(page, NOTE_PLACEHOLDER).fill('');
    await page.getByText('Simpan Catatan', { exact: true }).filter({ visible: true }).click();

    await expect(field(page, NOTE_PLACEHOLDER)).toBeVisible();
    expect(await noteViaApi(request, session.token, queue.id, 1)).toBe('asli');
  });
});
