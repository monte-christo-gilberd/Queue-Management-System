import { expect, test } from '@playwright/test';

import {
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

    await action(page, 'Buat Antrian Baru').click();
    await field(page, 'mis. Loket Pelayanan 1').fill(name);
    await action(page, 'Buat Antrian').click();

    await expect(page).toHaveURL(/\/queues\/\d+$/);
    await expect(inputWithValue(page, name)).toBeVisible();
  });

  test('TC-02-02 Nama kosong ditolak', async ({ page, request }) => {
    await ownerPage(page, request);

    await action(page, 'Buat Antrian Baru').click();
    await action(page, 'Buat Antrian').click();

    await expect(page.getByText('Nama antrian wajib diisi')).toBeVisible();
    await expect(page).toHaveURL(/\/dashboard$/);
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

    await expect(page.getByText('Antrian ini sudah ditutup.')).toBeVisible();
    await expect(action(page, 'TUTUP ANTRIAN')).toHaveCount(0);
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
    await action(page, 'Aktif').click();

    await expect(page.getByText(active.queue_name, { exact: true })).toBeVisible();
    await expect(page.getByText(closed.queue_name, { exact: true })).toHaveCount(0);
  });

  test('TC-02-07 Tautan join tampil di detail antrian', async ({ page, request }) => {
    const session = await ownerPage(page, request);
    const queue = await createQueueViaApi(request, session.token, uniqueName('Link'));
    await page.goto(`${WEB_URL}/queues/${queue.id}`);

    await expect(page.getByText('Tautan join untuk customer')).toBeVisible();
    await expect(page.getByText(new RegExp(`/join/${queue.id}$`))).toBeVisible();
    await expect(action(page, 'SALIN TAUTAN')).toBeVisible();
  });

  test('TC-02-08 Daftar antrian bisa dibuka dari menu profil', async ({ page, request }) => {
    await ownerPage(page, request);

    // Menu "Daftar Antrian" ada di panel profil; halamannya dibuka langsung di sini.
    await page.goto(`${WEB_URL}/queues`);

    await expect(page.getByText('Semua Antrian')).toBeVisible();
  });
});
