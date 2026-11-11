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
  API_URL,
} from './support';

test.describe('TS-04 Langganan owner', () => {
  test('TC-04-01 Owner tanpa langganan tidak bisa membuat antrian', async ({ page, request }) => {
    const account = await registerAccount(request, 'nosub');
    await signInWithToken(page, account.token, account.email);

    // Akun tanpa langganan adalah customer di UI: tombol buat antrian tidak ditampilkan.
    await expect(action(page, 'Masuk Ke Antrian')).toBeVisible();
    await expect(action(page, 'Buat Antrian Baru')).toHaveCount(0);

    // Backend tetap menolak pembuatan antrian dengan 402.
    const res = await request.post(`${API_URL}/api/v1/queues`, {
      headers: { Authorization: `Bearer ${account.token}` },
      data: { queue_name: uniqueName('Blokir'), queue_description: '' },
    });
    expect(res.status()).toBe(402);
  });

  test('TC-04-02 Owner tanpa langganan tetap bisa membuka daftar antrian', async ({ page, request }) => {
    const account = await registerAccount(request, 'list');
    await signInWithToken(page, account.token, account.email, '/queues');

    await expect(page.getByText('Daftar Antrian', { exact: true }).first()).toBeVisible();
    await expect(page.getByText(/Langganan Anda tidak aktif/)).toHaveCount(0);
  });

  test('TC-04-03 Antrian lama tetap berjalan setelah langganan kedaluwarsa', async ({ page, request }, testInfo) => {
    testInfo.skip(!hasExpiredOwner, 'Set QMS_EXPIRED_OWNER_EMAIL dan QMS_EXPIRED_OWNER_PASSWORD.');
    const token = await loginViaApi(request, EXPIRED_OWNER.email, EXPIRED_OWNER.password);
    // Antrian yang sudah ada dibuat sebelum langganan kedaluwarsa (disiapkan di DB).
    await signInWithToken(page, token, EXPIRED_OWNER.email);

    await expect(page.getByText('Belum ada antrian yang aktif')).toHaveCount(0);
    await expect(action(page, 'Next').first()).toBeVisible();
  });
});
