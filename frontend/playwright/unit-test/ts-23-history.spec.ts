import { expect, test } from '@playwright/test';

import {
  API_URL,
  auth,
  closedQueueWithAllOutcomes,
  createQueueViaApi,
  historyList,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  type HistoryQueue,
  type HistoryTicket,
} from './support';

test.describe('TS-23 Riwayat antrian owner (4.15)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-23-01 Antrian yang ditutup muncul di riwayat dengan ringkasan benar; antrian aktif tidak', async ({ request }) => {
    const { owner, queue } = await closedQueueWithAllOutcomes(request);

    const list = await historyList(request, owner.token);
    const item = list.queues.find((q) => q.id === queue.id);
    expect(item).toBeTruthy();
    expect(item).toMatchObject({ total: 6, completed: 2, skipped: 1, cancelled: 1, deleted: 1, unserved: 1 });
    expect(Date.parse(item!.closed_at)).toBeGreaterThan(Date.now() - 60_000);
    expect(list.queues[0].id).toBe(queue.id); // terbaru dulu
  });

  test('TC-23-02 Detail memuat nomor, customer, hasil akhir, dan catatan owner', async ({ request }) => {
    const { owner, queue, customers } = await closedQueueWithAllOutcomes(request);

    const res = await request.get(`${API_URL}/api/v1/history/queues/${queue.id}`, { headers: auth(owner.token) });
    expect(res.status()).toBe(200);
    const data = (await res.json()).data as { queue: HistoryQueue; tickets: HistoryTicket[]; next_cursor: string | null };
    expect(data.next_cursor).toBeNull();
    expect(data.tickets.map((t) => t.ticket_number)).toEqual([1, 2, 3, 4, 5, 6]);
    expect(data.tickets.map((t) => t.outcome)).toEqual(['skipped', 'completed', 'completed', 'cancelled', 'deleted', 'unserved']);
    expect(data.tickets[0].customer_email).toBe(customers[0].email);
    expect(data.tickets[2].note).toBe('minta struk');
    expect(data.tickets[1].note).toBeNull();
    expect(data.tickets[2].counter_name).toBeNull(); // satu counter: nama disembunyikan
    expect(data.tickets[2].completed_at).not.toBeNull(); // nomor yang dilayani saat ditutup menjadi selesai
  });

  test('TC-23-03 Bukan owner mendapat daftar kosong dan 404 pada detail; antrian aktif 404', async ({ request }) => {
    const { queue, customers } = await closedQueueWithAllOutcomes(request);
    const stranger = customers[0].token;

    expect((await historyList(request, stranger)).queues).toEqual([]);
    const detail = await request.get(`${API_URL}/api/v1/history/queues/${queue.id}`, { headers: auth(stranger) });
    expect(detail.status()).toBe(404);

    const owner = await subscribedOwnerSession(request);
    const active = await createQueueViaApi(request, owner!.token, uniqueName('Aktif'));
    const activeDetail = await request.get(`${API_URL}/api/v1/history/queues/${active.id}`, { headers: auth(owner!.token) });
    expect(activeDetail.status()).toBe(404);
  });

  test('TC-23-04 Paginasi daftar dan detail mengikuti kursor tanpa duplikat; kursor rusak 400', async ({ request }) => {
    const { owner, queue } = await closedQueueWithAllOutcomes(request);
    await closedQueueWithAllOutcomes(request); // antrian tertutup kedua, lebih baru

    const first = await historyList(request, owner.token, '?limit=1');
    expect(first.queues).toHaveLength(1);
    expect(first.next_cursor).not.toBeNull();
    const second = await historyList(request, owner.token, `?limit=1&cursor=${encodeURIComponent(first.next_cursor!)}`);
    expect(second.queues).toHaveLength(1);
    expect(second.queues[0].id).not.toBe(first.queues[0].id);
    expect(Date.parse(second.queues[0].closed_at)).toBeLessThanOrEqual(Date.parse(first.queues[0].closed_at));

    const seen: number[] = [];
    let cursor: string | null = null;
    for (let page = 0; page < 5; page++) {
      const q: string = `?limit=2${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`;
      const res = await request.get(`${API_URL}/api/v1/history/queues/${queue.id}${q}`, { headers: auth(owner.token) });
      const body = (await res.json()).data as { tickets: HistoryTicket[]; next_cursor: string | null };
      seen.push(...body.tickets.map((t) => t.ticket_number));
      cursor = body.next_cursor;
      if (!cursor) break;
    }
    expect(seen).toEqual([1, 2, 3, 4, 5, 6]);

    const bad = await request.get(`${API_URL}/api/v1/history/queues?cursor=abc`, { headers: auth(owner.token) });
    expect(bad.status()).toBe(400);
  });

  test('TC-23-05 UI: riwayat menampilkan ringkasan, detail menampilkan hasil dan catatan', async ({ page, request }) => {
    const { owner, name } = await closedQueueWithAllOutcomes(request);
    await signInWithToken(page, owner.token, owner.displayName, '/history');

    await expect(page.getByText(name, { exact: true })).toBeVisible();
    await expect(
      page.getByText('6 nomor · 2 selesai · 1 dilewati · 1 dibatalkan · 1 dihapus · 1 belum dilayani').first(),
    ).toBeVisible();

    await page.getByRole('button', { name: `Lihat riwayat ${name}` }).click();
    await expect(page.getByText('Semua Nomor Antrian')).toBeVisible();
    await expect(page.getByText('Selesai', { exact: true })).toHaveCount(2);
    await expect(page.getByText('Dilewati', { exact: true })).toHaveCount(1);
    await expect(page.getByText('Dibatalkan', { exact: true })).toHaveCount(1);
    await expect(page.getByText('Dihapus', { exact: true })).toHaveCount(1);
    await expect(page.getByText('Belum dilayani', { exact: true })).toHaveCount(1);
    await expect(page.getByText('Catatan: minta struk')).toBeVisible();
  });
});
