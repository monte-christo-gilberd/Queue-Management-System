import { expect, test } from '@playwright/test';

import {
  action,
  field,
  registerAccount,
  signInWithToken,
  uniqueName,
  WEB_URL,
  EXPIRED_OWNER,
  hasExpiredOwner,
  loginViaApi,
} from './support';

test.describe('TS-04 Langganan owner', () => {
  test('TC-04-01 Owner tanpa langganan tidak bisa membuat antrian', async ({ page, request }) => {
    const account = await registerAccount(request, 'nosub');
    await signInWithToken(page, account.token, account.email);

    await action(page, 'Buat Antrian Baru').click();
    await field(page, 'mis. Loket Pelayanan 1').fill(uniqueName('Blokir'));
    await action(page, 'Buat Antrian').click();

    await expect(page.getByText(/Langganan Anda tidak aktif/)).toBeVisible();
    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(page.getByText('Belum ada antrian yang aktif')).toBeVisible();
  });

  test('TC-04-02 Owner tanpa langganan tetap bisa membuka daftar antrian', async ({ page, request }) => {
    const account = await registerAccount(request, 'list');
    await signInWithToken(page, account.token, account.email, '/queues');

    await expect(page.getByText('Semua Antrian')).toBeVisible();
    await expect(page.getByText(/Langganan Anda tidak aktif/)).toHaveCount(0);
  });

  test('TC-04-03 Antrian lama tetap berjalan setelah langganan kedaluwarsa', async ({ page, request }, testInfo) => {
    testInfo.skip(!hasExpiredOwner, 'Set QMS_EXPIRED_OWNER_EMAIL dan QMS_EXPIRED_OWNER_PASSWORD.');
    const token = await loginViaApi(request, EXPIRED_OWNER.email, EXPIRED_OWNER.password);
    // Antrian yang sudah ada dibuat sebelum langganan kedaluwarsa (disiapkan di DB).
    await signInWithToken(page, token, EXPIRED_OWNER.email);

    await expect(page.getByText('Belum ada antrian yang aktif')).toHaveCount(0);
    await expect(action(page, 'NEXT')).toBeVisible();
  });

  test('TC-04-04 Opsi paket dan pembayaran tampil saat diblokir', async () => {
    test.fixme(true, 'Backlog: 4.18 — halaman harga/pembayaran belum ada (lihat test-cases/queue-management/backlog).');
  });
});
