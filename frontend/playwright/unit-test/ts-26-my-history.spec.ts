import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  closeQueueViaApi,
  createQueueViaApi,
  joinQueueViaApi,
  listTicketsViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

type Row = { ticket_id: number; queue_id: number; queue_name: string; outcome: string; queue_closed: boolean; ticket_number: number };

const SECRET_NOTE = 'catatan pribadi owner jangan bocor';

async function ticketId(request: APIRequestContext, token: string, queueId: number, number: number) {
  const rows = (await (await listTicketsViaApi(request, token, queueId)).json()).data.tickets as { id: number; ticket_number: number }[];
  const found = rows.find((t) => t.ticket_number === number);
  if (!found) throw new Error(`tiket ${number} tidak ada`);
  return found.id;
}

/**
 * Satu customer (c) dengan tiket di lima antrian, dibuat berurutan:
 *  A selesai dilayani, B dibatalkan sendiri, C belum dilayani saat antrian ditutup, D dihapus owner,
 *  E masih menunggu di antrian aktif (bukan riwayat). Customer lain (other) hanya ada di A.
 */
async function setup(request: APIRequestContext) {
  const owner = await subscribedOwnerSession(request);
  if (!owner) throw new Error('owner berlangganan tidak tersedia');
  const c = await registerAccount(request, 'hist');
  const other = await registerAccount(request, 'other');
  const names: Record<string, string> = {};
  const queues: Record<string, number> = {};

  const make = async (key: string) => {
    names[key] = uniqueName(`Riw${key}`);
    const q = await createQueueViaApi(request, owner.token, names[key]);
    queues[key] = q.id;
    return q.id;
  };

  const a = await make('A');
  await joinQueueViaApi(request, c.token, a); // A-001
  await joinQueueViaApi(request, other.token, a); // A-002
  expect((await nextViaApi(request, owner.token, a)).status()).toBe(200); // c dilayani
  expect((await nextViaApi(request, owner.token, a)).status()).toBe(200); // c selesai
  const noted = await request.put(`${API_URL}/api/v1/queues/${a}/tickets/${await ticketId(request, owner.token, a, 1)}/note`, {
    headers: auth(owner.token),
    data: { note: SECRET_NOTE },
  });
  expect(noted.status()).toBeLessThan(300);

  const b = await make('B');
  await joinQueueViaApi(request, c.token, b);
  const cancel = await request.post(`${API_URL}/api/v1/queues/${b}/cancel`, { headers: auth(c.token) });
  expect(cancel.status()).toBe(200);

  const cq = await make('C');
  await joinQueueViaApi(request, c.token, cq);
  await closeQueueViaApi(request, owner.token, cq);

  const d = await make('D');
  await joinQueueViaApi(request, c.token, d);
  const removed = await request.post(`${API_URL}/api/v1/queues/${d}/tickets/${await ticketId(request, owner.token, d, 1)}/remove`, { headers: auth(owner.token) });
  expect(removed.status()).toBe(200);

  const e = await make('E');
  await joinQueueViaApi(request, c.token, e); // aktif: bukan riwayat

  return { owner, c, other, names, queues };
}

async function myHistory(request: APIRequestContext, token: string, query = '') {
  const res = await request.get(`${API_URL}/api/v1/users/me/history${query}`, { headers: auth(token) });
  expect(res.status()).toBe(200);
  return (await res.json()).data as { tickets: Row[]; next_cursor: string | null };
}

test.describe('TS-26 Riwayat tiket customer', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-26-01 Satu baris per tiket, terbaru dulu, hasil akhir benar, tiket aktif tidak ikut', async ({ request }) => {
    const { c, names } = await setup(request);

    const { tickets, next_cursor } = await myHistory(request, c.token);
    expect(next_cursor).toBeNull();
    expect(tickets.map((t) => t.queue_name)).toEqual([names.D, names.C, names.B, names.A]);
    expect(tickets.map((t) => t.outcome)).toEqual(['deleted', 'unserved', 'cancelled', 'completed']);
    expect(tickets.map((t) => t.queue_closed)).toEqual([false, true, false, false]);
    expect(tickets.some((t) => t.queue_name === names.E)).toBe(false); // masih menunggu di antrian aktif
  });

  test('TC-26-02 Hanya tiket sendiri dan tanpa catatan owner', async ({ request }) => {
    const { c, other, names } = await setup(request);

    const mine = await request.get(`${API_URL}/api/v1/users/me/history`, { headers: auth(c.token) });
    const text = await mine.text();
    expect(text).not.toContain(SECRET_NOTE);
    expect(text).not.toContain('"note"');
    expect(text).not.toContain(other.email);

    // Customer lain hanya punya tiket aktif di A: riwayatnya kosong dan tidak memuat tiket c.
    const theirs = await myHistory(request, other.token);
    expect(theirs.tickets).toEqual([]);
    expect(JSON.stringify(theirs)).not.toContain(names.B);
  });

  test('TC-26-03 Paginasi mengikuti kursor tanpa duplikat; kursor rusak 400; tanpa login 401', async ({ request }) => {
    const { c } = await setup(request);

    const first = await myHistory(request, c.token, '?limit=2');
    expect(first.tickets).toHaveLength(2);
    expect(first.next_cursor).not.toBeNull();
    const second = await myHistory(request, c.token, `?limit=2&cursor=${encodeURIComponent(first.next_cursor!)}`);
    expect(second.tickets).toHaveLength(2);
    expect(second.next_cursor).toBeNull();

    const ids = [...first.tickets, ...second.tickets].map((t) => t.ticket_id);
    expect(new Set(ids).size).toBe(4);

    expect((await request.get(`${API_URL}/api/v1/users/me/history?cursor=abc`, { headers: auth(c.token) })).status()).toBe(400);
    expect((await request.get(`${API_URL}/api/v1/users/me/history`)).status()).toBe(401);
  });

  test('TC-26-04 UI: daftar riwayat dan popup detail tiket', async ({ page, request }) => {
    const { c, names } = await setup(request);
    await signInWithToken(page, c.token, c.email, '/riwayat');

    await expect(page.getByText(names.A, { exact: true })).toBeVisible();
    await expect(page.getByText(names.B, { exact: true })).toBeVisible();
    await expect(page.getByText(names.C, { exact: true })).toBeVisible();
    await expect(page.getByText(names.D, { exact: true })).toBeVisible();
    await expect(page.getByText(names.E, { exact: true })).toHaveCount(0); // tiket aktif bukan riwayat

    await page.getByRole('button', { name: new RegExp(`^Detail tiket A-001 ${names.A}$`) }).click();
    await expect(action(page, 'Detail Tiket')).toBeVisible();
    await expect(action(page, 'Sudah dilayani')).toBeVisible();
    await expect(action(page, 'Lama menunggu')).toBeVisible();
    await expect(page.getByText(SECRET_NOTE)).toHaveCount(0);
    await action(page, 'Tutup').click();
    await expect(action(page, 'Detail Tiket')).toBeHidden();

    await page.getByRole('button', { name: new RegExp(`^Detail tiket A-001 ${names.C}$`) }).click();
    await expect(action(page, 'Tidak sempat dilayani')).toBeVisible();
    await expect(page.getByText('Antrian ini sudah ditutup.')).toBeVisible();
  });

  test('TC-26-05 UI: customer tanpa riwayat melihat keadaan kosong', async ({ page, request }) => {
    const fresh = await registerAccount(request, 'kosong');
    await signInWithToken(page, fresh.token, fresh.email, '/riwayat');
    await expect(page.getByText('Belum Ada Riwayat')).toBeVisible();
  });
});
