import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  closeQueueViaApi,
  createQueueViaApi,
  field,
  joinQueueViaApi,
  registerAccount,
  signInThroughUi,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
  WEB_URL,
} from './support';

async function newQueue(request: APIRequestContext) {
  const session = await subscribedOwnerSession(request);
  if (!session) throw new Error('owner berlangganan tidak tersedia');
  const queue = await createQueueViaApi(request, session.token, uniqueName('Join'));
  return { ...queue, ownerToken: session.token };
}

test.describe('TS-05 Bergabung ke antrian (customer)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const session = await subscribedOwnerSession(request);
    testInfo.skip(session === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
  });

  test('TC-05-01 Tautan join meminta login lalu kembali', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'joinlogin');

    await page.goto(`${WEB_URL}/join/${queue.id}`);
    await expect(page).toHaveURL(new RegExp(`next=%2Fjoin%2F${queue.id}`));

    // Isi form di halaman yang sedang terbuka; signInThroughUi akan membuka "/" lagi dan menghapus next.
    await field(page, 'EMAIL ATAU USERNAME').fill(customer.email);
    await field(page, 'PASSWORD').fill(customer.password);
    await action(page, 'Login').click();

    await expect(page).toHaveURL(new RegExp(`/join/${queue.id}$`));
    await expect(action(page, 'Gabung Antrian')).toBeVisible();
  });

  test('TC-05-02 Daftar lewat tautan join kembali ke halaman join', async ({ page, request }) => {
    const queue = await newQueue(request);
    const username = uniqueName('baruj');

    await page.goto(`${WEB_URL}/join/${queue.id}`);
    await action(page, 'Daftar').click();
    await field(page, 'USERNAME').fill(username);
    await field(page, 'EMAIL').fill(`${username}@qms-test.local`);
    await field(page, 'PASSWORD').fill('Password123!');
    await field(page, 'KONFIRMASI PASSWORD').fill('Password123!');
    await action(page, 'Daftar').click();

    await expect(page).toHaveURL(new RegExp(`/join/${queue.id}$`));
    await expect(action(page, 'Gabung Antrian')).toBeVisible();
  });

  test('TC-05-03 Bergabung memberi nomor di akhir antrian', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'first');
    await signInWithToken(page, customer.token, customer.email, `/join/${queue.id}`);

    await action(page, 'Gabung Antrian').click();

    await expect(page.getByText('Nomor Anda')).toBeVisible();
    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
    await expect(page.getByText('Orang di depan Anda')).toBeVisible();
  });

  test('TC-05-04 Tautan yang sama tidak membuat nomor baru', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'again');
    await joinQueueViaApi(request, customer.token, queue.id);

    await signInWithToken(page, customer.token, customer.email, `/join/${queue.id}`);

    await expect(page.getByText('A-001', { exact: true })).toBeVisible();
    await expect(action(page, 'Gabung Antrian')).toHaveCount(0);
  });

  test('TC-05-05 Antrian tertutup tidak menerima pendaftaran', async ({ page, request }) => {
    const queue = await newQueue(request);
    await closeQueueViaApi(request, queue.ownerToken, queue.id);
    const customer = await registerAccount(request, 'closed');

    await signInWithToken(page, customer.token, customer.email, `/join/${queue.id}`);
    // Customer yang belum bergabung tidak bisa membaca detail antrian (backend), jadi penutupan
    // baru terlihat saat mencoba bergabung.
    await action(page, 'Gabung Antrian').click();

    await expect(page.getByText('Antrian sudah ditutup')).toBeVisible();
    await expect(page.getByText('Nomor Anda')).toHaveCount(0);
  });

  test('TC-05-06 Tautan tidak valid menampilkan antrian tidak ditemukan', async ({ page, request }) => {
    const customer = await registerAccount(request, 'badlink');
    await signInWithToken(page, customer.token, customer.email, '/join/abc');

    await expect(page.getByText('Antrian tidak ditemukan')).toBeVisible();
  });

  test('TC-05-07 Gabung ke antrian yang tidak ada ditolak', async ({ page, request }) => {
    const customer = await registerAccount(request, 'missing');
    await signInWithToken(page, customer.token, customer.email, '/join/999999999');

    await action(page, 'Gabung Antrian').click();

    await expect(page.getByText('Antrian tidak ditemukan')).toBeVisible();
  });

  test('TC-05-08 Masukkan tautan secara manual membuka halaman join', async ({ page, request }) => {
    const queue = await newQueue(request);
    const customer = await registerAccount(request, 'manual');
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();
    await action(page, 'Masukkan Link Manual').click();
    await field(page, 'https://...').fill(`${WEB_URL}/join/${queue.id}`);
    await action(page, 'Gabung Antrian').click();

    await expect(page).toHaveURL(new RegExp(`/join/${queue.id}$`));
  });

  test('TC-05-09 Tautan manual yang salah ditolak', async ({ page, request }) => {
    const customer = await registerAccount(request, 'badmanual');
    await signInWithToken(page, customer.token, customer.email);

    await action(page, 'Masuk Ke Antrian').click();
    await action(page, 'Masukkan Link Manual').click();
    await field(page, 'https://...').fill('bukan-tautan');
    await action(page, 'Gabung Antrian').click();

    await expect(page.getByText(/Tautan tidak valid/)).toBeVisible();
    await expect(page).toHaveURL(/\/dashboard$/);
  });
});
