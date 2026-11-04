import { expect, test, type APIRequestContext, type Browser, type Page } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  closeQueueViaApi,
  createQueueViaApi,
  joinQueueViaApi,
  nextViaApi,
  registerAccount,
  signInWithToken,
  skipViaApi,
  subscribedOwnerSession,
  uniqueName,
  WEB_URL,
} from './support';

type Counter = { id: number; name: string; is_default: boolean };

async function newQueue(request: APIRequestContext, prefix = 'Layar') {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName(prefix));
  return { owner: session.token, displayName: session.displayName, queue };
}

async function enableDisplay(request: APIRequestContext, owner: string, queueId: number, counts: object = {}) {
  const res = await request.put(`${API_URL}/api/v1/queues/${queueId}/display`, { headers: auth(owner), data: counts });
  expect(res.status(), await res.text()).toBe(200);
  return ((await res.json()) as { data: { token: string } }).data.token;
}

async function counters(request: APIRequestContext, owner: string, queueId: number): Promise<Counter[]> {
  const res = await request.get(`${API_URL}/api/v1/queues/${queueId}/counters`, { headers: auth(owner) });
  return ((await res.json()) as { data: { counters: Counter[] } }).data.counters;
}

async function addCounter(request: APIRequestContext, owner: string, queueId: number, name?: string): Promise<Counter> {
  const res = await request.post(`${API_URL}/api/v1/queues/${queueId}/counters`, { headers: auth(owner), data: name ? { name } : {} });
  expect(res.status(), await res.text()).toBe(201);
  return ((await res.json()) as { data: Counter }).data;
}

async function nextOn(request: APIRequestContext, owner: string, queueId: number, counterId: number) {
  const res = await request.post(`${API_URL}/api/v1/queues/${queueId}/next`, { headers: auth(owner), data: { counter_id: counterId } });
  expect(res.status(), await res.text()).toBe(200);
}

/** Layar publik di konteks browser BARU: tanpa sesi login sama sekali. */
async function openDisplay(browser: Browser, token: string, viewport?: { width: number; height: number }): Promise<Page> {
  const context = await browser.newContext(viewport ? { viewport } : {});
  const page = await context.newPage();
  await page.goto(`${WEB_URL}/display/${token}`);
  return page;
}

const cards = (page: Page) => page.getByTestId('display-card');

async function join(request: APIRequestContext, owner: string, queueId: number, n: number) {
  for (let i = 0; i < n; i++) {
    const acc = await registerAccount(request, 'disp');
    await joinQueueViaApi(request, acc.token, queueId, owner);
  }
}

test.describe('TS-32 Layar publik antrian (fitur 4.5)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    testInfo.skip((await subscribedOwnerSession(request)) === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-32-01 Owner mengaktifkan layar dari detail antrian; tautannya terbuka tanpa login', async ({ page, request, browser }) => {
    const { owner, displayName, queue } = await newQueue(request);
    await signInWithToken(page, owner, displayName, `/queues/${queue.id}`);

    await action(page, 'AKTIFKAN LAYAR').click();
    const link = page.getByLabel('Tautan layar');
    await expect(link).toBeVisible();
    const url = (await link.textContent()) ?? '';
    expect(url).toMatch(/\/display\/[A-Za-z0-9_-]{32}$/);

    const screen = await openDisplay(browser, new URL(url).pathname.split('/').pop()!);
    await expect(action(screen, queue.queue_name)).toBeVisible();
    await expect(screen).toHaveURL(/\/display\//);
    await expect(action(screen, 'Menunggu panggilan')).toBeVisible();
    await screen.context().close();
  });

  test('TC-32-02 Next dan Skip memperbarui layar di dua layar sekaligus tanpa memuat ulang', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id, { upcoming_count: 5 });
    await join(request, owner, queue.id, 3);
    const a = await openDisplay(browser, token);
    const b = await openDisplay(browser, token);
    for (const p of [a, b]) await expect(action(p, 'Berikutnya')).toBeVisible();

    await nextViaApi(request, owner, queue.id);
    for (const p of [a, b]) {
      await expect(cards(p).first()).toContainText('A-001', { timeout: 8000 });
      await expect(p.getByText('A-002', { exact: true })).toBeVisible();
      await expect(p.getByText('A-003', { exact: true })).toBeVisible();
    }

    // Skip: A-001 dilewati, A-002 dipanggil. Dengan fallback "remove" A-001 menjadi "dilewati".
    await skipViaApi(request, owner, queue.id);
    for (const p of [a, b]) await expect(cards(p).first()).toContainText('A-002', { timeout: 8000 });
    await a.context().close();
    await b.context().close();
  });

  test('TC-32-03 Satu counter: satu kartu tanpa nama counter', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    await join(request, owner, queue.id, 1);
    await nextViaApi(request, owner, queue.id);
    const page = await openDisplay(browser, token);

    await expect(cards(page)).toHaveCount(1);
    await expect(cards(page).first()).toContainText('A-001');
    await expect(cards(page).first()).not.toContainText('Counter 1');
    await page.context().close();
  });

  test('TC-32-04 Counter ditambah, diganti nama, dan dihapus langsung mengubah kartu di layar yang terbuka', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    const page = await openDisplay(browser, token);
    await expect(cards(page)).toHaveCount(1);

    const second = await addCounter(request, owner, queue.id, 'Loket Cepat');
    await expect(cards(page)).toHaveCount(2, { timeout: 8000 });
    await expect(page.getByText('Loket Cepat', { exact: true })).toBeVisible();
    await expect(page.getByText('Counter 1', { exact: true })).toBeVisible();

    const renamed = await request.put(`${API_URL}/api/v1/queues/${queue.id}/counters/${second.id}`, {
      headers: auth(owner),
      data: { name: 'Loket Baru' },
    });
    expect(renamed.status()).toBe(200);
    await expect(page.getByText('Loket Baru', { exact: true })).toBeVisible({ timeout: 8000 });

    const removed = await request.delete(`${API_URL}/api/v1/queues/${queue.id}/counters/${second.id}`, { headers: auth(owner) });
    expect(removed.status()).toBe(200);
    await expect(cards(page)).toHaveCount(1, { timeout: 8000 });
    await expect(page.getByText('Counter 1', { exact: true })).toHaveCount(0);
    await page.context().close();
  });

  test('TC-32-05 Dua counter melayani nomor berbeda; counter kosong tampil redup bernama', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    const second = await addCounter(request, owner, queue.id, 'Loket Cepat');
    const third = await addCounter(request, owner, queue.id, 'Loket Kosong');
    await join(request, owner, queue.id, 2);
    const [first] = await counters(request, owner, queue.id);
    await nextOn(request, owner, queue.id, first.id);
    await nextOn(request, owner, queue.id, second.id);
    const page = await openDisplay(browser, token);

    await expect(cards(page)).toHaveCount(3);
    await expect(cards(page).nth(0)).toContainText('A-001');
    await expect(cards(page).nth(1)).toContainText('A-002');
    await expect(cards(page).nth(1)).toContainText('Loket Cepat');
    await expect(cards(page).nth(2)).toContainText('Loket Kosong');
    await expect(cards(page).nth(2)).toContainText('Menunggu panggilan');
    expect(third.id).toBeGreaterThan(second.id);
    await page.screenshot({ path: 'playwright/.results/display-3-counters.png' });
    await page.context().close();
  });

  test('TC-32-06 Jumlah 0 menyembunyikan bagian itu sepenuhnya', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id, { upcoming_count: 0, skipped_count: 0 });
    await join(request, owner, queue.id, 3);
    await nextViaApi(request, owner, queue.id);
    const page = await openDisplay(browser, token);

    await expect(cards(page).first()).toContainText('A-001');
    await expect(page.getByText('Berikutnya', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Dilewati', { exact: true })).toHaveCount(0);
    await expect(page.getByText('A-002', { exact: true })).toHaveCount(0);

    await enableDisplay(request, owner, queue.id, { upcoming_count: 2, skipped_count: 1 });
    await expect(page.getByText('Berikutnya', { exact: true })).toBeVisible({ timeout: 8000 });
    await expect(page.getByText('A-002', { exact: true })).toBeVisible();
    await page.context().close();
  });

  test('TC-32-07 Ganti tautan mematikan yang lama (layar terbuka pun) dan matikan layar membuat 404', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const oldToken = await enableDisplay(request, owner, queue.id);
    const page = await openDisplay(browser, oldToken);
    await expect(action(page, queue.queue_name)).toBeVisible();

    const rotated = await request.post(`${API_URL}/api/v1/queues/${queue.id}/display/rotate`, { headers: auth(owner) });
    expect(rotated.status()).toBe(200);
    const newToken = ((await rotated.json()) as { data: { token: string } }).data.token;
    expect(newToken).not.toBe(oldToken);

    await expect(action(page, 'Tautan layar tidak berlaku')).toBeVisible({ timeout: 8000 });
    expect((await request.get(`${API_URL}/api/v1/display/${oldToken}`)).status()).toBe(404);
    expect((await request.get(`${API_URL}/api/v1/display/${newToken}`)).status()).toBe(200);

    const off = await request.delete(`${API_URL}/api/v1/queues/${queue.id}/display`, { headers: auth(owner) });
    expect(off.status()).toBe(204);
    expect((await request.get(`${API_URL}/api/v1/display/${newToken}`)).status()).toBe(404);
    await page.context().close();
  });

  test('TC-32-08 Antrian ditutup: layar menampilkan pesan penutupan', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    await join(request, owner, queue.id, 1);
    await nextViaApi(request, owner, queue.id);
    const page = await openDisplay(browser, token);
    await expect(cards(page).first()).toContainText('A-001');

    await closeQueueViaApi(request, owner, queue.id);
    await expect(action(page, 'Antrian ditutup')).toBeVisible({ timeout: 8000 });
    await expect(cards(page).first()).not.toContainText('A-001');
    await page.context().close();
  });

  test('TC-32-09 Sambungan putus menampilkan "Menyambung ulang..." dan pulih sendiri', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    await join(request, owner, queue.id, 1);
    const context = await browser.newContext();
    const page = await context.newPage();
    // Emulasi offline Playwright tidak memutus stream yang sudah terbuka, jadi kerusakan dibuat per permintaan:
    // stream SSE tidak pernah tersambung (reconnect terus), lalu pengambilan snapshot ikut diputus.
    await page.route('**/api/v1/display/*/event', (route) => route.abort());
    await page.goto(`${WEB_URL}/display/${token}`);
    await expect(action(page, queue.queue_name)).toBeVisible();
    // Tanpa stream, polling 3 detik menjaga status tersambung.
    await expect(action(page, 'Menyambung ulang...')).toHaveCount(0);

    await page.route('**/api/v1/display/*', (route) => route.abort());
    await expect(action(page, 'Menyambung ulang...')).toBeVisible({ timeout: 20_000 });
    await expect(action(page, queue.queue_name), 'angka terakhir tetap tampil, hanya ditandai').toBeVisible();

    // Selama putus, nomor berganti; setelah pulih layar menampilkan yang terbaru.
    await nextViaApi(request, owner, queue.id);
    await page.unroute('**/api/v1/display/*');
    await expect(action(page, 'Menyambung ulang...')).toHaveCount(0, { timeout: 15_000 });
    await expect(cards(page).first()).toContainText('A-001', { timeout: 15_000 });
    await context.close();
  });

  test('TC-32-10 Sepuluh counter muat di layar TV 1920x1080 tanpa terpotong atau saling menimpa', async ({ request, browser }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    for (let i = 0; i < 9; i++) await addCounter(request, owner, queue.id);
    await join(request, owner, queue.id, 10);
    for (const c of await counters(request, owner, queue.id)) await nextOn(request, owner, queue.id, c.id);
    const page = await openDisplay(browser, token, { width: 1920, height: 1080 });
    await expect(cards(page)).toHaveCount(10);
    await expect(cards(page).nth(9)).toContainText('A-010');

    const boxes = [];
    for (let i = 0; i < 10; i++) {
      const box = await cards(page).nth(i).boundingBox();
      expect(box, `kartu ${i}`).not.toBeNull();
      boxes.push(box!);
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(1920);
      expect(box!.y + box!.height).toBeLessThanOrEqual(1080);
    }
    for (let i = 0; i < boxes.length; i++) {
      for (let j = i + 1; j < boxes.length; j++) {
        const a = boxes[i];
        const b = boxes[j];
        const overlap = a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
        expect(overlap, `kartu ${i} dan ${j} tidak boleh saling menimpa`).toBe(false);
      }
    }
    await page.screenshot({ path: 'playwright/.results/display-10-counters.png' });
    await page.context().close();
  });

  test('TC-32-11 Respons publik tidak memuat id user, id tiket, owner, atau catatan', async ({ request }) => {
    const { owner, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id, { upcoming_count: 5, skipped_count: 5 });
    const acc = await registerAccount(request, 'privacy');
    await joinQueueViaApi(request, acc.token, queue.id, owner);
    await nextViaApi(request, owner, queue.id);
    const tickets = await request.get(`${API_URL}/api/v1/queues/${queue.id}/tickets`, { headers: auth(owner) });
    const t = ((await tickets.json()) as { data: { tickets: { id: number }[] } }).data.tickets[0];
    await request.put(`${API_URL}/api/v1/queues/${queue.id}/tickets/${t.id}/note`, { headers: auth(owner), data: { note: 'RAHASIA-OWNER' } });

    const res = await request.get(`${API_URL}/api/v1/display/${token}`);
    expect(res.status()).toBe(200);
    const text = await res.text();
    for (const secret of ['user_id', 'owner_id', 'ticket_id', 'RAHASIA-OWNER', acc.username, acc.email, 'sort_key']) {
      expect(text, secret).not.toContain(secret);
    }
    expect(Object.keys(((JSON.parse(text) as { data: object }).data))).toEqual(
      expect.arrayContaining(['queue_name', 'counters', 'upcoming', 'skipped', 'status', 'multi_counter']),
    );
  });

  test('TC-32-12 Token sembarang, salah bentuk, atau milik layar yang belum diaktifkan: 404 yang sama', async ({ request }) => {
    const { owner, queue } = await newQueue(request);
    for (const token of ['x', 'A'.repeat(32), 'bukan-token-bukan-token-bukan-toke', '../etc/passwd']) {
      const res = await request.get(`${API_URL}/api/v1/display/${encodeURIComponent(token)}`);
      expect(res.status(), token).toBe(404);
    }
    const stream = await request.get(`${API_URL}/api/v1/display/${'A'.repeat(32)}/event`);
    expect(stream.status()).toBe(404);
    // Belum diaktifkan: owner mendapat 404 pada konfigurasi.
    const cfg = await request.get(`${API_URL}/api/v1/queues/${queue.id}/display`, { headers: auth(owner) });
    expect(cfg.status()).toBe(404);
  });

  test('TC-32-13 Hanya owner antrian yang bisa mengatur layar; jumlah di luar 0-10 ditolak', async ({ request }) => {
    const { owner, queue } = await newQueue(request);
    const stranger = await registerAccount(request, 'stranger');
    const put = (token: string, data: object) =>
      request.put(`${API_URL}/api/v1/queues/${queue.id}/display`, { headers: auth(token), data });

    expect((await put(stranger.token, {})).status()).toBe(404);
    expect((await request.get(`${API_URL}/api/v1/queues/${queue.id}/display`)).status()).toBe(401);
    expect((await put(owner, { upcoming_count: 11 })).status()).toBe(400);
    expect((await put(owner, { skipped_count: -1 })).status()).toBe(400);
    expect((await put(owner, { upcoming_count: 10, skipped_count: 10 })).status()).toBe(200);
    expect((await request.post(`${API_URL}/api/v1/queues/${queue.id}/display/rotate`, { headers: auth(stranger.token) })).status()).toBe(404);
  });

  test('TC-32-14 Owner mengatur jumlah dengan tombol + dan -; tersimpan di server', async ({ page, request }) => {
    const { owner, displayName, queue } = await newQueue(request);
    await enableDisplay(request, owner, queue.id, { upcoming_count: 5, skipped_count: 0 });
    await signInWithToken(page, owner, displayName, `/queues/${queue.id}`);

    await page.getByLabel('Tambah nomor berikutnya').click();
    await expect(page.getByLabel('nomor berikutnya: 6')).toBeVisible();
    await page.getByLabel('Tambah nomor dilewati').click();
    await expect(page.getByLabel('nomor dilewati: 1')).toBeVisible();

    const cfg = await request.get(`${API_URL}/api/v1/queues/${queue.id}/display`, { headers: auth(owner) });
    const data = ((await cfg.json()) as { data: { upcoming_count: number; skipped_count: number } }).data;
    expect(data.upcoming_count).toBe(6);
    expect(data.skipped_count).toBe(1);

    for (let i = 0; i < 2; i++) await page.getByLabel('Kurangi nomor dilewati').click({ force: true });
    await expect(page.getByLabel('nomor dilewati: 0')).toBeVisible();
    await expect(page.getByLabel('Kurangi nomor dilewati')).toBeDisabled();
  });

  test('TC-32-15 Ganti tautan dan matikan meminta konfirmasi; Batal tidak mengubah apa pun', async ({ page, request }) => {
    const { owner, displayName, queue } = await newQueue(request);
    const token = await enableDisplay(request, owner, queue.id);
    await signInWithToken(page, owner, displayName, `/queues/${queue.id}`);

    await action(page, 'GANTI TAUTAN').click();
    await expect(action(page, 'Ganti tautan layar?')).toBeVisible();
    await action(page, 'Batal').click();
    expect((await request.get(`${API_URL}/api/v1/display/${token}`)).status()).toBe(200);

    await action(page, 'MATIKAN LAYAR').click();
    await action(page, 'Ya, Matikan').click();
    await expect(action(page, 'AKTIFKAN LAYAR')).toBeVisible();
    expect((await request.get(`${API_URL}/api/v1/display/${token}`)).status()).toBe(404);
  });
});
