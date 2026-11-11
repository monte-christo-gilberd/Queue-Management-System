import { expect, test, type Page } from '@playwright/test';

import { action, field, WEB_URL } from './support';

/**
 * TS-35 Pesan konfirmasi password di halaman Daftar dan Reset Password.
 *
 * Aturan yang diuji: pesan "Password tidak sama" tampil sebagai teks di bawah field konfirmasi (bukan toast), hanya
 * setelah field itu ditinggalkan (blur) atau tombol kirim ditekan, dan hilang sendiri begitu isinya sama. Selama
 * pengguna masih mengetik di field konfirmasi, pesan tidak boleh muncul: karakter pertama selalu berbeda dari password.
 *
 * Tes tidak membutuhkan Mailpit dan tidak membuat akun: semua penolakan terjadi di sisi klien sebelum permintaan ke server.
 */

/** Teks pesan inline konfirmasi password. */
const MISMATCH = 'Password tidak sama';

/** Teks toast galat saat tombol kirim ditekan dengan konfirmasi yang berbeda. */
const SUBMIT_ERROR = 'Password dan konfirmasi password harus sama';

/** Pesan inline di bawah field konfirmasi, dicari persis agar tidak tertukar dengan toast galat kirim. */
function mismatch(page: Page) {
  return page.getByText(MISMATCH, { exact: true });
}

test.describe('TS-35 Pesan konfirmasi password', () => {
  test('TC-35-01 Daftar: mengetik di field konfirmasi tidak memunculkan pesan', async ({ page }) => {
    await page.goto(`${WEB_URL}/register`);
    await field(page, 'PASSWORD').fill('Password123!');
    const confirm = field(page, 'KONFIRMASI PASSWORD');

    // Ketikan per karakter seperti pengguna sungguhan: sejak karakter pertama isinya sudah berbeda dari password.
    await confirm.click();
    await confirm.pressSequentially('Pass', { delay: 50 });
    await page.waitForTimeout(300);
    await expect(mismatch(page)).toHaveCount(0);

    // Melanjutkan sampai sama persis: tetap tidak ada pesan selama fokus masih di field konfirmasi.
    await confirm.pressSequentially('word123!', { delay: 50 });
    await expect(confirm).toHaveValue('Password123!');
    await expect(mismatch(page)).toHaveCount(0);
  });

  test('TC-35-02 Daftar: pesan muncul setelah field ditinggalkan dan hilang saat isinya sama', async ({ page }) => {
    await page.goto(`${WEB_URL}/register`);
    await field(page, 'PASSWORD').fill('Password123!');
    const confirm = field(page, 'KONFIRMASI PASSWORD');

    await confirm.fill('Password999!');
    await expect(mismatch(page)).toHaveCount(0);
    await confirm.blur();
    await expect(mismatch(page)).toBeVisible();

    // Memperbaiki isi sampai sama: pesan hilang sendiri tanpa perlu blur lagi.
    await confirm.fill('Password123!');
    await expect(mismatch(page)).toHaveCount(0);
  });

  test('TC-35-03 Daftar: setelah field ditinggalkan, mengubah password mengevaluasi ulang kesamaannya', async ({ page }) => {
    await page.goto(`${WEB_URL}/register`);
    const password = field(page, 'PASSWORD');
    const confirm = field(page, 'KONFIRMASI PASSWORD');

    await password.fill('Password123!');
    await confirm.fill('Password123!');
    await confirm.blur();
    await expect(mismatch(page)).toHaveCount(0);

    // Password diubah sehingga tidak lagi sama dengan konfirmasi: pesan muncul karena field konfirmasi sudah pernah ditinggalkan.
    await password.fill('Password456!');
    await expect(mismatch(page)).toBeVisible();

    // Konfirmasi disamakan lagi: pesan hilang.
    await confirm.fill('Password456!');
    await expect(mismatch(page)).toHaveCount(0);
  });

  test('TC-35-04 Daftar: field konfirmasi kosong tidak memunculkan pesan walau ditinggalkan', async ({ page }) => {
    await page.goto(`${WEB_URL}/register`);
    await field(page, 'PASSWORD').fill('Password123!');
    const confirm = field(page, 'KONFIRMASI PASSWORD');

    await confirm.click();
    await confirm.blur();
    await page.waitForTimeout(300);
    await expect(mismatch(page)).toHaveCount(0);
  });

  test('TC-35-05 Daftar: menekan Daftar dengan konfirmasi berbeda menampilkan pesan inline dan toast galat kirim', async ({ page }) => {
    await page.goto(`${WEB_URL}/register`);
    await field(page, 'USERNAME').fill('pengguna-uji');
    await field(page, 'EMAIL').fill('pengguna-uji@qms-test.local');
    await field(page, 'PASSWORD').fill('Password123!');
    await field(page, 'KONFIRMASI PASSWORD').fill('Password999!');

    await action(page, 'Daftar').click();

    // Dua pesan berbeda: teks inline di bawah field dan toast dari validasi kirim. Pengguna tetap di halaman daftar.
    await expect(mismatch(page)).toBeVisible();
    await expect(page.getByText(SUBMIT_ERROR, { exact: true })).toBeVisible();
    await expect(page).toHaveURL(/\/register$/);
  });

  test('TC-35-06 Reset password: perilaku pesan sama dengan halaman Daftar', async ({ page }) => {
    // Token sembarang tetap menampilkan form sampai kata sandi dikirim (lihat TC-31-07).
    await page.goto(`${WEB_URL}/reset-password?token=token-sembarang`);
    await field(page, 'PASSWORD BARU').fill('PasswordBaru1!');
    const confirm = field(page, 'KONFIRMASI PASSWORD');

    // Mengetik: tidak ada pesan walau isinya baru sebagian.
    await confirm.click();
    await confirm.pressSequentially('Pass', { delay: 50 });
    await page.waitForTimeout(300);
    await expect(mismatch(page)).toHaveCount(0);

    // Meninggalkan field: pesan tampil. Menyamakan isi: pesan hilang.
    await confirm.blur();
    await expect(mismatch(page)).toBeVisible();
    await confirm.fill('PasswordBaru1!');
    await expect(mismatch(page)).toHaveCount(0);

    // Menekan Simpan Password dengan konfirmasi berbeda: pesan inline dan toast galat kirim, form tetap tampil.
    await confirm.fill('PasswordLain1!');
    await action(page, 'Simpan Password').click();
    await expect(mismatch(page)).toBeVisible();
    await expect(page.getByText(SUBMIT_ERROR, { exact: true })).toBeVisible();
    await expect(action(page, 'Simpan Password')).toBeVisible();
  });
});
