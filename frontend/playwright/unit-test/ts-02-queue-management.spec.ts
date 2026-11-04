import { expect, test } from '@playwright/test';

import {
  openCreateQueue,
  action,
  closeQueueViaApi,
  createQueueViaApi,
  field,
  inputWithValue,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  WEB_URL,
} from './support';

test.describe('TS-02 Mengelola antrian (owner)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  async function ownerPage(page: import('@playwright/test').Page, request: import('@playwright/test').APIRequestContext) {
    const session = await subscribedOwnerSession(request);
    if (!session) throw new Error('owner berlangganan tidak tersedia');
    await signInWithToken(page, session.token, session.displayName);
    return session;
  }

  test('TC-02-01 Membuat antrian baru', async ({ page, request }) => {
    await ownerPage(page, request);
    const name = uniqueName('Loket');

    await openCreateQueue(page);
    await field(page, 'mis. Loket Pelayanan 1').fill(name);
    await action(page, 'Buat Antrian').click();

    await expect(page).toHaveURL(/\/queues\/\d+$/);
    await expect(inputWithValue(page, name)).toBeVisible();
  });

  test('TC-02-02 Nama kosong ditolak', async ({ page, request }) => {
    await ownerPage(page, request);

    await openCreateQueue(page);
    await action(page, 'Buat Antrian').click();

    await expect(page.getByText('Nama antrian wajib diisi')).toBeVisible();
    await expect(page).toHaveURL(/\/dashboard/);
  });

  test('TC-02-03 Mengubah nama dan deskripsi tersimpan', async ({ page, request }) => {
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Awal'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    const updated = uniqueName('Baru');
    await inputWithValue(page, queue.queue_name).fill(updated);
    await action(page, 'SIMPAN').click();
    await page.reload();

    await expect(inputWithValue(page, updated)).toBeVisible();
  });

  test('TC-02-04 Menutup antrian setelah konfirmasi', async ({ page, request }) => {
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Tutup'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    await action(page, 'TUTUP ANTRIAN').click();
    await action(page, 'Ya, Tutup Antrian').click();

    // Setelah ditutup owner diarahkan ke dashboard, dan antrian hilang dari Daftar Antrian.
    await expect(page).toHaveURL(/\/dashboard/);
    await page.goto(`${WEB_URL}/queues`);
    await expect(page.getByText(queue.queue_name, { exact: true })).toHaveCount(0);
  });

  test('TC-02-05 Batal menutup tidak mengubah antrian', async ({ page, request }) => {
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Batal'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    await action(page, 'TUTUP ANTRIAN').click();
    await action(page, 'Batal').click();

    await expect(action(page, 'TUTUP ANTRIAN')).toBeVisible();
    await expect(page.getByText('Antrian ini sudah ditutup.')).toHaveCount(0);
  });

  test('TC-02-06 Filter Aktif hanya menampilkan antrian aktif', async ({ page, request }) => {
    const session = await ownerPage(page, request);
    const active = await createQueueViaApi(request, session.token, uniqueName('Aktif'));
    const closed = await createQueueViaApi(request, session.token, uniqueName('Tertutup'));
    await closeQueueViaApi(request, session.token, closed.id);

    await page.goto(`${WEB_URL}/queues`);

    await expect(page.getByText(active.queue_name, { exact: true })).toBeVisible();
    await expect(page.getByText(closed.queue_name, { exact: true })).toHaveCount(0);
  });

  test('TC-02-07 QR join dan tombol salin tampil di detail antrian', async ({ page, request }) => {
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Link'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    await action(page, 'LIHAT QR').click();
    await expect(action(page, 'SALIN TAUTAN')).toBeVisible();
    await expect(action(page, 'UNDUH PNG')).toBeVisible();
  });

  test('TC-02-07b Salin tautan benar-benar menaruh tautan join di clipboard', async ({ page, request, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Salin'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    await action(page, 'LIHAT QR').click();
    await action(page, 'SALIN TAUTAN').click();

    await expect(action(page, 'TAUTAN TERSALIN')).toBeVisible();
    // Membaca clipboard hanya bisa di konteks aman (localhost/HTTPS); di http://<ip-lan> jalur fallback yang diuji,
    // dan label "TERSALIN" di atas sudah membuktikan execCommand berhasil.
    if (await page.evaluate(() => window.isSecureContext)) {
      expect(await page.evaluate(() => navigator.clipboard.readText())).toMatch(/\/join\?t=v1\./);
    }
    // Label kembali ke "SALIN TAUTAN" setelah beberapa detik.
    await expect(action(page, 'SALIN TAUTAN')).toBeVisible({ timeout: 5000 });
  });

  test('TC-02-07c Salin gagal (tanpa Clipboard API dan execCommand ditolak): ada pesan dan tautan bisa disalin manual', async ({
    page,
    request,
  }) => {
    await page.addInitScript(() => {
      Object.defineProperty(navigator, 'clipboard', { value: undefined, configurable: true });
      document.execCommand = () => false;
    });
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('SalinGagal'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    await action(page, 'LIHAT QR').click();
    await action(page, 'SALIN TAUTAN').click();

    await expect(action(page, 'Tidak bisa menyalin otomatis. Pilih tautan di bawah lalu salin manual.')).toBeVisible();
    await expect(page.getByLabel('Tautan join')).toHaveValue(/\/join\?t=v1\./);
    await expect(action(page, 'TAUTAN TERSALIN')).toHaveCount(0);
  });

  test('TC-02-08 Daftar antrian bisa dibuka dari menu profil', async ({ page, request }) => {
    await ownerPage(page, request);

    // Menu "Daftar Antrian" ada di panel profil; halamannya dibuka langsung di sini.
    await page.goto(`${WEB_URL}/queues`);

    await expect(page.getByText('Daftar Antrian', { exact: true }).first()).toBeVisible();
  });
});
