import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  createQueueViaApi,
  joinQueueViaApi,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

const API_URL = process.env.QMS_API_URL ?? 'http://localhost:8080';

type Session = { token: string; displayName: string };
type CounterRow = { id: number; name: string; is_default: boolean; serving: { ticket_number: number } | null };
type TicketRow = { id: number; ticket_number: number; status: string; skip_count: number; called_counter_id?: number | null };

/** Server membungkus payload dalam `data` (lihat tes Skip: `.data.ticket_number`). */
function unwrap<T>(body: T & { data?: T }): T {
  return body.data ?? body;
}

function auth(token: string) {
  return { Authorization: `Bearer ${token}` };
}

async function createCounter(request: APIRequestContext, token: string, queueId: number): Promise<CounterRow> {
  const res = await request.post(`${API_URL}/api/v1/queues/${queueId}/counters`, { headers: auth(token), data: {} });
  expect(res.status()).toBe(201);
  return unwrap(await res.json());
}

async function listCounters(request: APIRequestContext, token: string, queueId: number): Promise<CounterRow[]> {
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/counters`, { headers: auth(token) });
  expect(res.status()).toBe(200);
  return unwrap(await res.json()).counters as CounterRow[];
}

async function nextOn(request: APIRequestContext, token: string, queueId: number, counterId: number, expected?: number) {
  return request.post(`${API_URL}/api/v1/queues/${queueId}/next`, {
    headers: auth(token),
    data: { counter_id: counterId, expected_ticket_id: expected },
  });
}

async function tickets(request: APIRequestContext, token: string, queueId: number): Promise<TicketRow[]> {
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/tickets`, { headers: auth(token) });
  expect(res.status()).toBe(200);
  return unwrap(await res.json()).tickets as TicketRow[];
}

async function setup(request: APIRequestContext, customers: number) {
  const session = (await subscribedOwnerSession(request)) as Session | null;
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Counter'));
  const customerTokens: string[] = [];
  for (let i = 0; i < customers; i++) {
    const account = await registerAccount(request, 'counter');
    await joinQueueViaApi(request, account.token, queue.id);
    customerTokens.push(account.token);
  }
  const defaultCounter = (await listCounters(request, session.token, queue.id)).find((c) => c.is_default);
  if (!defaultCounter) throw new Error('counter default tidak ada');
  return { session, queue, defaultId: defaultCounter.id, customerTokens };
}

test.describe('TS-19 Multi-counter (4.11 / US-4)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-19-01 Dua counter melayani dua nomor berbeda bersamaan', async ({ request }) => {
    const { session, queue, defaultId, customerTokens } = await setup(request, 3);
    const second = await createCounter(request, session.token, queue.id);

    expect((await nextOn(request, session.token, queue.id, defaultId)).status()).toBe(200);
    expect((await nextOn(request, session.token, queue.id, second.id)).status()).toBe(200);

    // Status dibaca customer (owner tidak punya tiket, jadi status owner 404).
    const status = await request.get(`${API_URL}/api/v1/queues/${queue.id}/status`, { headers: auth(customerTokens[0]) });
    const serving = unwrap(await status.json()).serving as { counter_id: number; ticket_number: number }[];
    const byCounter = Object.fromEntries(serving.map((s) => [s.counter_id, s.ticket_number]));
    expect(byCounter[defaultId]).toBe(1);
    expect(byCounter[second.id]).toBe(2);
  });

  test('TC-19-02 Next di satu counter tidak mengubah nomor counter lain', async ({ request }) => {
    const { session, queue, defaultId } = await setup(request, 3);
    const second = await createCounter(request, session.token, queue.id);
    await nextOn(request, session.token, queue.id, defaultId);
    await nextOn(request, session.token, queue.id, second.id);

    // expected_ticket_id berisi ID tiket (bukan nomor) yang sedang dilayani counter default.
    const before = Object.fromEntries((await tickets(request, session.token, queue.id)).map((t) => [t.ticket_number, t]));
    // Counter default menyelesaikan nomor 1 dan menerima nomor 3; counter 2 tetap melayani nomor 2.
    expect((await nextOn(request, session.token, queue.id, defaultId, before[1].id)).status()).toBe(200);
    const rows = await tickets(request, session.token, queue.id);
    const byNumber = Object.fromEntries(rows.map((t) => [t.ticket_number, t]));
    expect(byNumber[1].status).toBe('completed');
    expect(byNumber[3].status).toBe('currently_served');
    expect(byNumber[3].called_counter_id).toBe(defaultId);
    expect(byNumber[2].status).toBe('currently_served');
    expect(byNumber[2].called_counter_id).toBe(second.id);
  });

  test('TC-19-03 Aksi dengan nomor yang sudah berubah ditolak 409 tanpa perubahan (D9)', async ({ request }) => {
    const { session, queue, defaultId } = await setup(request, 2);
    await nextOn(request, session.token, queue.id, defaultId); // counter melayani nomor 1
    const before = await tickets(request, session.token, queue.id);

    // Perangkat lain sudah memajukan counter; perangkat ini masih mengira nomor 0 (kosong).
    const res = await nextOn(request, session.token, queue.id, defaultId, 0);
    expect(res.status()).toBe(409);
    expect((await res.json()).error.code).toBe('COUNTER_STATE_CHANGED');
    expect(await tickets(request, session.token, queue.id)).toEqual(before);
  });

  test('TC-19-04 Hapus counter mengembalikan nomornya ke urutan terdepan tanpa menaikkan skip', async ({ request }) => {
    const { session, queue } = await setup(request, 2);
    const second = await createCounter(request, session.token, queue.id);
    await nextOn(request, session.token, queue.id, second.id); // counter 2 melayani nomor 1

    const res = await request.delete(`${API_URL}/api/v1/queues/${queue.id}/counters/${second.id}`, {
      headers: auth(session.token),
    });
    expect(res.status()).toBeLessThan(300);

    const rows = await tickets(request, session.token, queue.id);
    const returned = rows.find((t) => t.ticket_number === 1);
    expect(returned?.status).toBe('waiting');
    expect(returned?.skip_count).toBe(0);
    expect(returned?.called_counter_id ?? null).toBeNull();
    expect((await listCounters(request, session.token, queue.id)).some((c) => c.id === second.id)).toBe(false);
  });

  test('TC-19-05 Tambah counter menampilkan popup nama dengan saran, lalu membuat tab baru', async ({ page, request }) => {
    const { session, queue } = await setup(request, 1);
    // Halaman detail hanya memuat antrian ini (dashboard memuat semua antrian aktif owner).
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);

    await expect(page.getByRole('button', { name: 'Tambah counter' })).toBeVisible();
    await page.getByRole('button', { name: 'Tambah counter' }).click();
    await expect(page.getByText('Counter baru', { exact: true })).toBeVisible();
    await expect(page.getByPlaceholder('Nama counter')).toHaveValue('Counter 2');
    await action(page, 'Buat counter').click();
    await expect(page.getByRole('tab', { name: /Counter 2/ })).toBeVisible();
  });

  test('TC-19-06 Popup tambah counter menerima nama sendiri', async ({ page, request }) => {
    const { session, queue } = await setup(request, 1);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);

    await page.getByRole('button', { name: 'Tambah counter' }).click();
    await page.getByPlaceholder('Nama counter').fill('Loket B');
    await action(page, 'Buat counter').click();
    await expect(page.getByRole('tab', { name: /Loket B/ })).toBeVisible();
  });

  test('TC-19-07 Ganti nama lewat popup tombol ✎ mengubah nama tab', async ({ page, request }) => {
    const { session, queue } = await setup(request, 1);
    await createCounter(request, session.token, queue.id);
    await signInWithToken(page, session.token, session.displayName, `/queues/${queue.id}`);

    await page.getByRole('button', { name: 'Ganti nama Counter 2' }).click();
    await expect(page.getByText('Ganti nama counter', { exact: true })).toBeVisible();
    await page.getByPlaceholder('Nama counter').fill('Loket Utama');
    await action(page, 'Simpan').click();
    await expect(page.getByRole('tab', { name: /Loket Utama/ })).toBeVisible();
  });
});
