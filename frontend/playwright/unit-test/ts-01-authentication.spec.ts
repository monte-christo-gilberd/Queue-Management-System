import { expect, test } from '@playwright/test';

import { registerAccount, uniqueName } from './support/api';
import { WEB_URL } from './support/env';
import { action, expectLoginScreen, field, signInThroughUi, signInWithToken } from './support/ui';

test.describe('TS-01 Authentication', () => {
  test('TC-01-01 Login dengan email', async ({ page, request }) => {
    const account = await registerAccount(request, 'login');

    await signInThroughUi(page, account.email, account.password);

    await expect(page).toHaveURL(/\/dashboard$/);
    // Akun customer tidak melihat bagian owner (antrian kosong dan tombol buat antrian).
    await expect(action(page, 'Masuk Ke Antrian')).toBeVisible();
    await expect(action(page, 'Buat Antrian Baru')).toHaveCount(0);
  });

  test('TC-01-02 Login dengan username', async ({ page, request }) => {
    const account = await registerAccount(request, 'uname');

    await signInThroughUi(page, account.username, account.password);

    await expect(page).toHaveURL(/\/dashboard$/);
  });

  test('TC-01-03 Password salah menampilkan pesan generik', async ({ page, request }) => {
    const account = await registerAccount(request, 'wrongpw');

    await signInThroughUi(page, account.email, 'salah-sekali');

    await expect(page.getByText('Email/username atau password salah')).toBeVisible();
    await expect(page).not.toHaveURL(/\/dashboard/);
  });

  test('TC-01-04 Identifier tidak terdaftar menampilkan pesan yang sama', async ({ page }) => {
    await signInThroughUi(page, `tidak-ada-${uniqueName('x')}`, 'apa-saja');

    await expect(page.getByText('Email/username atau password salah')).toBeVisible();
  });

  test('TC-01-05 Isian kosong ditolak di form', async ({ page }) => {
    await page.goto(`${WEB_URL}/`);
    await action(page, 'Login').click();

    await expect(page.getByText('Email/username dan password wajib diisi')).toBeVisible();
    await expectLoginScreen(page);
  });

  test('TC-01-06 Register akun baru masuk ke Dashboard', async ({ page }) => {
    const username = uniqueName('baru');
    await page.goto(`${WEB_URL}/register`);
    await field(page, 'USERNAME').fill(username);
    await field(page, 'EMAIL').fill(`${username}@qms-test.local`);
    await field(page, 'PASSWORD').fill('Password123!');
    await field(page, 'KONFIRMASI PASSWORD').fill('Password123!');
    await action(page, 'Daftar').click();

    await expect(page).toHaveURL(/\/dashboard$/);
    // Akun customer tidak melihat bagian owner (antrian kosong dan tombol buat antrian).
    await expect(action(page, 'Masuk Ke Antrian')).toBeVisible();
    await expect(action(page, 'Buat Antrian Baru')).toHaveCount(0);
  });

  test('TC-01-07 Email yang sudah terdaftar ditolak', async ({ page, request }) => {
    const existing = await registerAccount(request, 'dupe');

    await page.goto(`${WEB_URL}/register`);
    await field(page, 'USERNAME').fill(uniqueName('lain'));
    await field(page, 'EMAIL').fill(existing.email);
    await field(page, 'PASSWORD').fill('Password123!');
    await field(page, 'KONFIRMASI PASSWORD').fill('Password123!');
    await action(page, 'Daftar').click();

    await expect(page.getByText('Email sudah terdaftar')).toBeVisible();
    await expect(page).toHaveURL(/\/register$/);
  });

  test('TC-01-08 Konfirmasi password berbeda ditolak', async ({ page }) => {
    await page.goto(`${WEB_URL}/register`);
    await field(page, 'USERNAME').fill(uniqueName('beda'));
    await field(page, 'EMAIL').fill(`${uniqueName('beda')}@qms-test.local`);
    await field(page, 'PASSWORD').fill('Password123!');
    await field(page, 'KONFIRMASI PASSWORD').fill('Password999!');

    // Selama pengguna masih di field konfirmasi, pesan belum tampil; setelah fokus pindah, pesan muncul di bawah field.
    await expect(page.getByText('Password tidak sama')).toHaveCount(0);
    await field(page, 'KONFIRMASI PASSWORD').blur();
    await expect(page.getByText('Password tidak sama')).toBeVisible();
    await action(page, 'Daftar').click();
    await expect(page).toHaveURL(/\/register$/);
  });

  test('TC-01-09 Halaman terlindungi kembali setelah login', async ({ page, request }) => {
    const account = await registerAccount(request, 'back');

    await page.goto(`${WEB_URL}/queues`);
    await expect(page).toHaveURL(/\/\?next=%2Fqueues/);

    // Isi form di halaman yang sedang terbuka (jangan lewat signInThroughUi, karena itu membuka "/" lagi
    // dan menghapus parameter next).
    await field(page, 'EMAIL ATAU USERNAME').fill(account.email);
    await field(page, 'PASSWORD').fill(account.password);
    await action(page, 'Login').click();

    await expect(page).toHaveURL(/\/queues$/);
  });

  test('TC-01-10 Sesi tetap aktif setelah refresh', async ({ page, request }) => {
    const account = await registerAccount(request, 'refresh');
    await signInWithToken(page, account.token, account.email);

    await page.reload();

    await expect(page).toHaveURL(/\/dashboard$/);
    // Akun customer tidak melihat bagian owner (antrian kosong dan tombol buat antrian).
    await expect(action(page, 'Masuk Ke Antrian')).toBeVisible();
    await expect(action(page, 'Buat Antrian Baru')).toHaveCount(0);
  });

  test('TC-01-11 Logout menghapus sesi', async ({ page, request }) => {
    const account = await registerAccount(request, 'logout');
    await signInWithToken(page, account.token, account.email);
    // Akun customer tidak melihat bagian owner (antrian kosong dan tombol buat antrian).
    await expect(action(page, 'Masuk Ke Antrian')).toBeVisible();
    await expect(action(page, 'Buat Antrian Baru')).toHaveCount(0);

    // Ikon profil di header berada tepat setelah nama pengguna (lihat ResponsiveScreenLayout)
    await page.getByText(account.email, { exact: true }).locator('xpath=following-sibling::*[1]').click();
    await action(page, 'Keluar').click();

    await expectLoginScreen(page);
    await page.goto(`${WEB_URL}/dashboard`);
    await expectLoginScreen(page);
  });

  test('TC-01-12 Parameter next ke situs lain diabaikan', async ({ page, request }) => {
    const account = await registerAccount(request, 'redir');

    await page.goto(`${WEB_URL}/login?next=${encodeURIComponent('https://evil.example')}`);
    await field(page, 'EMAIL ATAU USERNAME').fill(account.email);
    await field(page, 'PASSWORD').fill(account.password);
    await action(page, 'Login').click();

    await expect(page).toHaveURL(/\/dashboard$/);
    expect(page.url().startsWith(WEB_URL)).toBe(true);
  });
});
