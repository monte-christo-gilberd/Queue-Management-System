import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  createQueueViaApi,
  joinQueueViaApi,
  listTicketsViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

const SECRET = 'rahasia owner jangan bocor';

async function ticketId(request: APIRequestContext, token: string, queueId: number, number: number) {
  const rows = (await (await listTicketsViaApi(request, token, queueId)).json()).data.tickets as { id: number; ticket_number: number }[];
  const found = rows.find((t) => t.ticket_number === number);
  if (!found) throw new Error(`tiket ${number} tidak ada`);
  return found.id;
}

/** Antrian aktif dengan dua customer; owner menulis catatan di nomor 1 lewat API. */
async function setup(request: APIRequestContext) {
  const owner = await subscribedOwnerSession(request);
  if (!owner) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, owner.token, uniqueName('Catatan'));
  const a = await registerAccount(request, 'note');
  const b = await registerAccount(request, 'note');
  await joinQueueViaApi(request, a.token, queue.id);
  await joinQueueViaApi(request, b.token, queue.id);
  const id1 = await ticketId(request, owner.token, queue.id, 1);
  const saved = await request.put(`${API_URL}/api/v1/queues/${queue.id}/tickets/${id1}/note`, { headers: auth(owner.token), data: { note: SECRET } });
  expect(saved.status()).toBeLessThan(300);
  return { owner, queue, a, b };
}

async function openTicketList(page: import('@playwright/test').Page, token: string, name: string, queueId: number) {
  await signInWithToken(page, token, name, `/queues/${queueId}`);
  await action(page, 'sedang dilayani').first().click();
  await expect(action(page, 'Daftar tiket')).toBeVisible();
}

test.describe('TS-25 Catatan tampil di daftar tiket (4.16)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-25-01 Daftar tiket owner membawa catatan, dan respons customer tidak pernah memuatnya', async ({ request }) => {
    const { owner, queue, a, b } = await setup(request);

    const list = (await (await listTicketsViaApi(request, owner.token, queue.id)).json()).data.tickets as { ticket_number: number; note: string | null; note_updated_at: string | null }[];
    expect(list.find((t) => t.ticket_number === 1)?.note).toBe(SECRET);
    expect(list.find((t) => t.ticket_number === 1)?.note_updated_at).not.toBeNull();
    expect(list.find((t) => t.ticket_number === 2)?.note).toBeNull();

    // Customer pemilik nomor bercatatan: tidak satu pun respons membawa isi atau kunci catatan.
    for (const token of [a.token, b.token]) {
      const status = await request.get(`${API_URL}/api/v1/queues/${queue.id}/status`, { headers: auth(token) });
      expect(status.status()).toBe(200);
      const statusText = await status.text();
      expect(statusText).not.toContain(SECRET);
      expect(statusText).not.toContain('"note"');

      const mine = await request.get(`${API_URL}/api/v1/users/me/tickets`, { headers: auth(token) });
      const mineText = await mine.text();
      expect(mineText).not.toContain(SECRET);
      expect(mineText).not.toContain('"note"');
    }
  });

  test('TC-25-02 Bagian Catatan menampilkan pratinjau, dan edit dari daftar memperbaruinya', async ({ page, request }) => {
    const { owner, queue } = await setup(request);
    await openTicketList(page, owner.token, owner.displayName, queue.id);

    await expect(page.getByText('Catatan', { exact: true })).toBeVisible();
    await expect(page.getByText(SECRET)).toBeVisible();

    await page.getByRole('button', { name: 'Edit catatan A-001' }).click();
    const box = page.getByPlaceholder('Tulis catatan untuk nomor ini...');
    await expect(box).toHaveValue(SECRET);
    await box.fill('catatan sudah diubah');
    await action(page, 'Simpan Catatan').click();

    await expect(page.getByText('catatan sudah diubah')).toBeVisible();
    await expect(page.getByText(SECRET)).toHaveCount(0);
  });

  test('TC-25-03 Menambah catatan untuk nomor yang belum punya dari daftar tiket', async ({ page, request }) => {
    const { owner, queue } = await setup(request);
    await openTicketList(page, owner.token, owner.displayName, queue.id);

    await action(page, 'Tambah catatan').last().click(); // A-002 belum punya catatan
    await page.getByPlaceholder('Tulis catatan untuk nomor ini...').fill('minta struk');
    await action(page, 'Append Note').last().click();

    await expect(page.getByRole('button', { name: 'Edit catatan A-002' })).toBeVisible();
    await expect(page.getByText('minta struk')).toBeVisible();
  });
});
