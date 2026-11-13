import { expect, test, type APIRequestContext, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { action, clickNext, API_URL, closeQueueViaApi, openCreateQueue, createQueueViaApi, field, joinPathViaApi, joinTokenOf, loginViaApi, WEB_URL } from './support';

/**
 * TS-10 — Skenario antrian end-to-end dengan akun seed (db/scripts/reset_seed_dev.sql).
 *
 * Akun: owner1 dan owner2 (owner), user1..user10 (customer), semua dengan password "password".
 * Prasyarat: database sudah di-reset dengan skrip seed, backend :8080 dan frontend :8081 berjalan.
 * Tes berjalan berurutan dan berbagi data (satu antrian utama "A"), dengan beberapa browser sekaligus
 * agar pembaruan real-time antara owner dan customer ikut diuji.
 */
const PASSWORD = 'password';
const OWNER1 = 'owner1@test.com';
const OWNER2 = 'owner2@test.com';
const user = (n: number) => `user${n}@test.com`;

const stamp = Date.now().toString(36);
const NAME_A = `Uji A ${stamp}`;
const NAME_B = `Uji B ${stamp}`;
const NAME_C = `Uji C ${stamp}`;
const NAME_D = `Uji D ${stamp}`;

/** Isi form login yang sedang tampil, lalu klik Login. */
async function submitLogin(page: Page, identifier: string, password = PASSWORD) {
  await field(page, 'EMAIL ATAU USERNAME').fill(identifier);
  await field(page, 'PASSWORD').fill(password);
  await action(page, 'Login').click();
}

/** Buka panel profil di header desktop (ikon di sebelah nama pengguna). */
async function openPanel(page: Page, displayName: string) {
  await page
    .getByText(displayName, { exact: true })
    .filter({ visible: true })
    .locator('xpath=following-sibling::*[1]')
    .click();
}

/** Nilai pada baris label-nilai di kartu status customer (mis. "Orang di depan Anda"). */
function rowValue(page: Page, label: string) {
  return page.getByText(label, { exact: true }).locator('xpath=following-sibling::*[1]');
}

/** Kartu "Nomor Anda" di halaman join customer. */
function ticketCard(page: Page) {
  return page.getByText('Nomor Anda', { exact: true }).locator('xpath=..');
}

/** Login owner/customer lewat UI pada halaman baru. */
async function loginPage(browser: Browser, identifier: string): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext();
  const page = await context.newPage();
  await page.goto(`${WEB_URL}/`);
  await submitLogin(page, identifier);
  await expect(page).toHaveURL(/\/dashboard/);
  return { context, page };
}

/** Path join `/join?t=<token>` antrian A. Token statis tidak berubah, jadi cukup diambil sekali. */
let joinPathA = '';
async function joinPath(request: APIRequestContext, queueId: number): Promise<string> {
  if (!joinPathA) joinPathA = await joinPathViaApi(request, queueId, await loginViaApi(request, OWNER1, PASSWORD));
  return joinPathA;
}

/**
 * Customer login lalu bergabung. Pemindaian kamera tidak bisa dijalankan di tes, jadi join
 * dikirim ke POST /join dengan token QR (yang sama dengan hasil scan), lalu halaman tiket dibuka.
 */
async function joinAs(page: Page, request: APIRequestContext, queueId: number, identifier: string) {
  await page.goto(`${WEB_URL}/`);
  await submitLogin(page, identifier);
  await expect(page).toHaveURL(/\/dashboard/);
  const token = await loginViaApi(request, identifier, PASSWORD);
  const res = await request.post(`${API_URL}/api/v1/join`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { t: joinTokenOf(await joinPath(request, queueId)) },
  });
  expect(res.status(), await res.text()).toBe(200);
  await page.goto(`${WEB_URL}/ticket/${queueId}`);
}

async function createViaUi(page: Page, ownerName: string, queueName: string): Promise<number> {
  await openCreateQueue(page);
  await field(page, 'mis. Loket Pelayanan 1').fill(queueName);
  await action(page, 'Buat Antrian').click();
  await page.waitForURL(/\/queues\/\d+/);
  return Number(page.url().match(/\/queues\/(\d+)/)![1]);
}

async function closeViaUi(page: Page, queueId: number) {
  await page.goto(`${WEB_URL}/queues/${queueId}`);
  await action(page, 'TUTUP ANTRIAN').click();
  await action(page, 'Ya, Tutup Antrian').click();
}

test.describe.configure({ mode: 'serial' });

// Skenario ini mengandaikan owner1 belum punya antrian aktif; tutup sisa dari tes lain.
test.beforeAll(async ({ request }) => {
  const token = await loginViaApi(request, OWNER1, 'password');
  const res = await request.get(`${API_URL}/api/v1/queues`, { headers: { Authorization: `Bearer ${token}` } });
  const body = (await res.json()) as { data: { queues: { id: number; status: string }[] } };
  for (const q of body.data.queues.filter((x) => x.status === 'active')) {
    await closeQueueViaApi(request, token, q.id);
  }
});

test.describe('TS-10 Skenario antrian dengan akun seed', () => {
  let owner: { context: BrowserContext; page: Page };
  let u1: { context: BrowserContext; page: Page };
  let u2: { context: BrowserContext; page: Page };
  let u3: { context: BrowserContext; page: Page };
  let queueA = 0;

  test.afterAll(async () => {
    for (const s of [owner, u1, u2, u3]) await s?.context.close();
  });

  /* ------------------------------- Autentikasi ------------------------------- */

  test('TC-10-01 Password salah: pesan tampil dan tetap di halaman login', async ({ page }) => {
    await page.goto(`${WEB_URL}/`);
    await submitLogin(page, OWNER1, 'salah-total');
    await expect(page.getByText('Email/username atau password salah')).toBeVisible();
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
  });

  test('TC-10-02 Akun tidak terdaftar: pesan sama, tidak membedakan mana yang salah', async ({ page }) => {
    await page.goto(`${WEB_URL}/`);
    await submitLogin(page, 'tidakada@test.com');
    await expect(page.getByText('Email/username atau password salah')).toBeVisible();
  });

  test('TC-10-03 Tanpa login: dashboard dan halaman join mengarahkan ke login', async ({ page }) => {
    await page.goto(`${WEB_URL}/dashboard`);
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
    await page.goto(`${WEB_URL}/join?t=x`);
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
  });

  test('TC-10-04 Login dengan username berhasil masuk dashboard', async ({ page }) => {
    await page.goto(`${WEB_URL}/`);
    await submitLogin(page, 'owner2');
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test('TC-10-05 Owner login dengan email: dashboard kosong dan menu buat antrian ada', async ({ browser }) => {
    owner = await loginPage(browser, OWNER1);
    await expect(owner.page.getByText('Belum ada antrian yang aktif')).toBeVisible();
    await openPanel(owner.page, OWNER1);
    await expect(action(owner.page, 'Buat Antrian Baru')).toBeVisible();
    await expect(action(owner.page, 'Daftar Antrian')).toBeVisible();
    await expect(action(owner.page, 'Riwayat Antrian')).toBeVisible();
  });

  /* ------------------------------ Owner: antrian ------------------------------ */

  test('TC-10-06 Buat antrian dengan nama kosong ditolak', async () => {
    await owner.page.goto(`${WEB_URL}/dashboard`);
    await openCreateQueue(owner.page);
    await action(owner.page, 'Buat Antrian').click();
    await expect(owner.page.getByText('Nama antrian wajib diisi')).toBeVisible();
    await action(owner.page, 'Batal').click();
  });

  test('TC-10-07 Buat antrian A lewat menu owner: halaman detail terbuka', async () => {
    await owner.page.goto(`${WEB_URL}/dashboard`);
    queueA = await createViaUi(owner.page, OWNER1, NAME_A);
    await expect(owner.page.locator(`input[value="${NAME_A}"]`)).toBeVisible();
    await action(owner.page, 'LIHAT QR').click();
    await expect(action(owner.page, 'SALIN TOKEN')).toBeVisible();
  });

  test('TC-10-08 Dashboard menampilkan antrian A; Next pada antrian kosong menampilkan pesan', async () => {
    await owner.page.goto(`${WEB_URL}/dashboard`);
    await expect(owner.page.getByText(NAME_A, { exact: true })).toBeVisible();
    await clickNext(owner.page);
    await expect(owner.page.getByText('Tidak ada yang menunggu di antrian ini')).toBeVisible();
  });

  /* ----------------------------- Customer: bergabung ----------------------------- */

  test('TC-10-09 user1 login lalu bergabung lewat token: nomor A-001 menunggu, 0 orang di depan', async ({ browser, request }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    u1 = { context, page };
    await joinAs(page, request, queueA, user(1));
    await expect(page.getByText(NAME_A, { exact: true })).toBeVisible();
    await expect(ticketCard(page)).toContainText('A-001');
    await expect(ticketCard(page)).toContainText('Menunggu');
    await expect(rowValue(page, 'Orang di depan Anda')).toHaveText('0');
    await expect(rowValue(page, 'Sedang dilayani')).toHaveText('-');
  });

  test('TC-10-10 Buka ulang tiket: nomor yang sama, tombol gabung tidak muncul, tidak ada tiket ganda', async ({ request }) => {
    await u1.page.goto(`${WEB_URL}/ticket/${queueA}`);
    await expect(ticketCard(u1.page)).toContainText('A-001');
    await expect(action(u1.page, 'Gabung Antrian')).toHaveCount(0);

    const token = await loginViaApi(request, user(1), PASSWORD);
    const joinAgain = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { t: joinTokenOf(await joinPath(request, queueA)) },
    });
    expect(joinAgain.status()).toBe(200);
    const again = ((await joinAgain.json()) as { data: { ticket_number: number } }).data;
    expect(again.ticket_number).toBe(1);

    const ownerToken = await loginViaApi(request, OWNER1, PASSWORD);
    const res = await request.get(`${API_URL}/api/v1/queues/${queueA}/tickets`, {
      headers: { Authorization: `Bearer ${ownerToken}` },
    });
    const body = (await res.json()) as { data: { tickets: unknown[] } };
    expect(body.data.tickets).toHaveLength(1);
  });

  test('TC-10-11 user2 dan user3 bergabung: nomor A-002 dan A-003, orang di depan 1 dan 2', async ({ browser, request }) => {
    for (const [n, label, ahead] of [[2, 'A-002', '1'], [3, 'A-003', '2']] as const) {
      const context = await browser.newContext();
      const page = await context.newPage();
      if (n === 2) u2 = { context, page };
      else u3 = { context, page };
      await joinAs(page, request, queueA, user(n));
      await expect(ticketCard(page)).toContainText(label);
      await expect(rowValue(page, 'Orang di depan Anda')).toHaveText(ahead);
    }
  });

  test('TC-10-12 Papan owner menampilkan tiga tiket menunggu secara live', async () => {
    await owner.page.goto(`${WEB_URL}/dashboard`);
    await expect(owner.page.getByText('A-001', { exact: true })).toBeVisible();
    await expect(owner.page.getByText('A-002', { exact: true })).toBeVisible();
    await expect(owner.page.getByText('A-003', { exact: true })).toBeVisible();
    await expect(owner.page.getByText('menunggu', { exact: true })).toHaveCount(3);
  });

  /* ------------------------ Akses yang tidak diizinkan ------------------------ */

  test('TC-10-13 Antrian milik owner lain atau customer: halaman detail menolak', async ({ browser }) => {
    const other = await loginPage(browser, OWNER2);
    await other.page.goto(`${WEB_URL}/queues/${queueA}`);
    await expect(other.page.getByText('Antrian tidak ditemukan')).toBeVisible();
    await other.context.close();

    await u1.page.goto(`${WEB_URL}/queues/${queueA}`);
    await expect(u1.page.getByText('Antrian tidak ditemukan')).toBeVisible();
  });

  test('TC-10-14 Customer tidak melihat menu owner di panel profil', async () => {
    await u1.page.goto(`${WEB_URL}/dashboard`);
    await openPanel(u1.page, user(1));
    await expect(action(u1.page, 'Beranda')).toBeVisible();
    await expect(action(u1.page, 'Buat Antrian Baru')).toHaveCount(0);
    await expect(action(u1.page, 'Daftar Antrian')).toHaveCount(0);
    await expect(action(u1.page, 'Riwayat Antrian')).toHaveCount(0);
  });

  test('TC-10-15 Halaman /join?t= hanya menampilkan petunjuk, tanpa tombol gabung', async ({ browser }) => {
    const { context, page } = await loginPage(browser, user(4));
    await page.goto(`${WEB_URL}/join?t=bukan-token`);
    await expect(page.getByText('Pindai QR di aplikasi')).toBeVisible();
    await expect(action(page, 'Gabung Antrian')).toHaveCount(0);
    await context.close();
  });

  /* ------------------------------ Next dan real-time ------------------------------ */

  test('TC-10-16 Next pertama: A-001 dilayani, customer melihatnya tanpa refresh', async ({ request }) => {
    // user1 kembali ke halaman join (tes sebelumnya memindahkannya) agar koneksi live-nya aktif.
    await u1.page.goto(`${WEB_URL}/ticket/${queueA}`);
    await expect(ticketCard(u1.page)).toContainText('A-001');

    await owner.page.goto(`${WEB_URL}/dashboard`);
    await clickNext(owner.page);
    await expect(owner.page.getByText('sedang dilayani', { exact: true })).toBeVisible();

    await expect(rowValue(u1.page, 'Sedang dilayani')).toHaveText('A-001', { timeout: 15_000 });
    await expect(ticketCard(u1.page)).toContainText('Sedang dilayani');
    await expect(rowValue(u2.page, 'Sedang dilayani')).toHaveText('A-001', { timeout: 15_000 });
    await expect(rowValue(u2.page, 'Orang di depan Anda')).toHaveText('0', { timeout: 15_000 });
    await expect(rowValue(u3.page, 'Orang di depan Anda')).toHaveText('1', { timeout: 15_000 });
  });

  test('TC-10-17 Next kedua: A-002 dilayani, A-001 menjadi sudah dilayani', async () => {
    await clickNext(owner.page);
    await expect(rowValue(u2.page, 'Sedang dilayani')).toHaveText('A-002', { timeout: 15_000 });
    await expect(ticketCard(u2.page)).toContainText('Sedang dilayani');
    await expect(ticketCard(u1.page)).toContainText('Sudah dilayani', { timeout: 15_000 });
    await expect(rowValue(u3.page, 'Orang di depan Anda')).toHaveText('0', { timeout: 15_000 });
  });

  test('TC-10-18 Tampilan customer sesuai kondisi: beranda, Antrian Saya, dan Riwayat', async () => {
    // user1 sudah dilayani dan nomornya lewat: tidak lagi di beranda/Antrian Saya, masuk Riwayat.
    await u1.page.goto(`${WEB_URL}/dashboard`);
    await expect(u1.page.getByText('Anda tidak sedang mengantri')).toBeVisible();
    await u1.page.goto(`${WEB_URL}/riwayat`);
    await expect(u1.page.getByText(NAME_A, { exact: true })).toBeVisible();
    // Baris riwayat memuat status, tanggal, dan jam dalam satu teks ("Sudah dilayani · 4 Nov 2026 · 16:49"); bisa ada beberapa.
    await expect(u1.page.getByText(/^Sudah dilayani/).first()).toBeVisible();

    // user2 sedang dilayani: kartu di beranda dan di Antrian Saya.
    await u2.page.goto(`${WEB_URL}/dashboard`);
    await expect(u2.page.getByText(NAME_A.toLowerCase(), { exact: true })).toBeVisible();
    await u2.page.goto(`${WEB_URL}/antrian`);
    await expect(u2.page.getByText(NAME_A, { exact: true })).toBeVisible();

    // user3 masih menunggu: ada di Antrian Saya dengan status Menunggu.
    await u3.page.goto(`${WEB_URL}/antrian`);
    await expect(u3.page.getByText(NAME_A, { exact: true })).toBeVisible();
    await expect(u3.page.getByText('Menunggu', { exact: true })).toBeVisible();
  });

  /* --------------------------- Tutup antrian: dashboard → riwayat --------------------------- */

  test('TC-10-19 Tutup antrian A: pindah ke dashboard dan antrian itu hilang dari sana', async () => {
    await closeViaUi(owner.page, queueA);
    await expect(owner.page).toHaveURL(/\/dashboard/);
    await expect(owner.page.getByText(NAME_A, { exact: true })).toHaveCount(0);
    await expect(owner.page.getByText('Belum ada antrian yang aktif')).toBeVisible();
  });

  test('TC-10-20 Owner: antrian A tidak ada di Daftar Antrian dan muncul di Riwayat Antrian', async () => {
    await owner.page.goto(`${WEB_URL}/queues`);
    await expect(owner.page.getByText(NAME_A, { exact: true })).toHaveCount(0);
    await expect(owner.page.getByText('Belum Ada Antrian Aktif')).toBeVisible();

    await owner.page.goto(`${WEB_URL}/history`);
    await expect(owner.page.getByText(NAME_A, { exact: true })).toBeVisible();
  });

  test('TC-10-21 Customer setelah ditutup: Antrian Saya kosong, Riwayat menandai antrian ditutup', async () => {
    // user3 masih menunggu saat antrian ditutup: pindah dari Antrian Saya ke Riwayat.
    await u3.page.goto(`${WEB_URL}/antrian`);
    await expect(u3.page.getByText('Belum Ikut Antrian')).toBeVisible();
    await u3.page.goto(`${WEB_URL}/riwayat`);
    await expect(u3.page.getByText(NAME_A, { exact: true })).toBeVisible();
    // Nomor yang masih menunggu saat antrian ditutup dicatat "Tidak sempat dilayani" (riwayat per tiket, item 4).
    await expect(u3.page.getByText(/^Tidak sempat dilayani/).first()).toBeVisible();

    await u2.page.goto(`${WEB_URL}/riwayat`);
    await expect(u2.page.getByText(/^Sudah dilayani/).first()).toBeVisible();
  });

  test('TC-10-22 Antrian ditutup: customer baru tidak bisa bergabung (UI dan API)', async ({ browser, request }) => {
    const token = await loginViaApi(request, user(4), PASSWORD);
    const res = await request.post(`${API_URL}/api/v1/join`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { t: joinTokenOf(await joinPath(request, queueA)) },
    });
    expect(res.status()).toBe(409);
    expect(((await res.json()) as { error: { code: string } }).error.code).toBe('QUEUE_CLOSED');
  });

  /* ------------------------------ Dashboard: dua terbaru ------------------------------ */

  test('TC-10-23 Dashboard hanya menampilkan dua antrian aktif terbaru, sisanya di Daftar Antrian', async ({ request }) => {
    const token = await loginViaApi(request, OWNER1, PASSWORD);
    await createQueueViaApi(request, token, NAME_B);
    await createQueueViaApi(request, token, NAME_C);
    await createQueueViaApi(request, token, NAME_D);

    await owner.page.goto(`${WEB_URL}/dashboard`);
    await expect(owner.page.getByText(NAME_D, { exact: true })).toBeVisible();
    await expect(owner.page.getByText(NAME_C, { exact: true })).toBeVisible();
    await expect(owner.page.getByText(NAME_B, { exact: true })).toHaveCount(0);

    await owner.page.goto(`${WEB_URL}/queues`);
    for (const name of [NAME_B, NAME_C, NAME_D]) {
      await expect(owner.page.getByText(name, { exact: true })).toBeVisible();
    }
    await expect(owner.page.getByText(NAME_A, { exact: true })).toHaveCount(0);
  });

  test('TC-10-24 Menutup antrian terbaru: yang tertunda naik ke dashboard, yang ditutup ke riwayat', async ({ request }) => {
    const token = await loginViaApi(request, OWNER1, PASSWORD);
    const res = await request.get(`${API_URL}/api/v1/queues`, { headers: { Authorization: `Bearer ${token}` } });
    const queues = ((await res.json()) as { data: { queues: { id: number; queue_name: string }[] } }).data.queues;
    const d = queues.find((q) => q.queue_name === NAME_D)!;

    await closeViaUi(owner.page, d.id);
    await expect(owner.page).toHaveURL(/\/dashboard/);
    await expect(owner.page.getByText(NAME_D, { exact: true })).toHaveCount(0);
    await expect(owner.page.getByText(NAME_C, { exact: true })).toBeVisible();
    await expect(owner.page.getByText(NAME_B, { exact: true })).toBeVisible();

    await owner.page.goto(`${WEB_URL}/history`);
    await expect(owner.page.getByText(NAME_D, { exact: true })).toBeVisible();
    await expect(owner.page.getByText(NAME_A, { exact: true })).toBeVisible();

    await owner.page.goto(`${WEB_URL}/queues`);
    await expect(owner.page.getByText(NAME_D, { exact: true })).toHaveCount(0);
    await expect(owner.page.getByText(NAME_C, { exact: true })).toBeVisible();
    await expect(owner.page.getByText(NAME_B, { exact: true })).toBeVisible();
  });

  test('TC-10-25 Owner lain tidak melihat antrian owner1 di dashboard, daftar, maupun riwayat', async ({ browser }) => {
    const other = await loginPage(browser, OWNER2);
    await expect(other.page.getByText('Belum ada antrian yang aktif')).toBeVisible();
    await other.page.goto(`${WEB_URL}/queues`);
    await expect(other.page.getByText('Belum Ada Antrian Aktif')).toBeVisible();
    await other.page.goto(`${WEB_URL}/history`);
    await expect(other.page.getByText('Belum Ada Riwayat')).toBeVisible();
    await other.context.close();
  });
});
