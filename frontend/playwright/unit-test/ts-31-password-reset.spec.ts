import { expect, test, type APIRequestContext } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  expectNoMail,
  field,
  mailpitAvailable,
  registerAccount,
  signInThroughUi,
  signInWithToken,
  waitForMail,
  WEB_URL,
} from './support';

const pause = (ms: number) => new Promise((r) => setTimeout(r, ms));
const SUBJECT = /password/i;

/** POST forgot-password. nginx membatasi 1 permintaan/detik per IP, jadi 429 dicoba ulang sebentar lagi. */
async function requestReset(request: APIRequestContext, email: string) {
  for (let i = 0; i < 6; i++) {
    const res = await request.post(`${API_URL}/api/v1/users/forgot-password`, { data: { email } });
    if (res.status() !== 429) return res;
    await pause(1200);
  }
  throw new Error('forgot-password terus 429');
}

/** Token dari email terbaru ke `email`. */
async function tokenFromMail(request: APIRequestContext, email: string, timeoutMs = 15_000): Promise<string> {
  const mail = await waitForMail(request, email, { subject: SUBJECT, timeoutMs });
  const link = mail.links.find((l) => l.includes('/reset-password?token='));
  if (!link) throw new Error(`email tanpa tautan reset: ${mail.text}`);
  return new URL(link).searchParams.get('token') ?? '';
}

async function resetViaApi(request: APIRequestContext, token: string, password: string) {
  return request.post(`${API_URL}/api/v1/users/reset-password`, { data: { token, password } });
}

async function errorCode(res: { json(): Promise<unknown> }): Promise<string> {
  return ((await res.json()) as { error?: { code?: string } }).error?.code ?? '';
}

// Dijalankan berurutan: laju forgot-password dibatasi per IP di nginx.
test.describe.configure({ mode: 'serial' });

test.describe('TS-31 Lupa dan reset password lewat email (plan item 9)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    testInfo.skip(!(await mailpitAvailable(request)), 'Mailpit tidak berjalan (docker compose up mailpit).');
  });

  test('TC-31-01 Registrasi menolak password kurang dari 8 karakter', async ({ request }) => {
    const res = await request.post(`${API_URL}/api/v1/users/register`, {
      data: { email: `short${Date.now()}@qms-test.local`, username: 'short', password: '1234567' },
    });
    expect(res.status()).toBe(400);
    expect(await errorCode(res)).toBe('WEAK_PASSWORD');
  });

  test('TC-31-02 Alur penuh lewat UI: minta tautan, buka dari email, simpan password, login, tautan tidak bisa dipakai lagi', async ({
    page,
    request,
  }) => {
    const acc = await registerAccount(request, 'reset');

    await page.goto(`${WEB_URL}/forgot-password`);
    await field(page, 'EMAIL').fill(acc.email);
    await action(page, 'Kirim').click();
    await expect(action(page, 'Cek Email Anda')).toBeVisible();

    const token = await tokenFromMail(request, acc.email);
    await page.goto(`${WEB_URL}/reset-password?token=${encodeURIComponent(token)}`);
    await field(page, 'PASSWORD BARU').fill('PasswordBaru1!');
    await field(page, 'KONFIRMASI PASSWORD').fill('PasswordBaru1!');
    await action(page, 'Simpan Password').click();
    await expect(action(page, 'Password Diganti')).toBeVisible();

    // Password lama tidak lagi berlaku, yang baru berlaku.
    const old = await request.post(`${API_URL}/api/v1/users/login`, { data: { identifier: acc.email, password: acc.password } });
    expect(old.status()).toBe(401);
    await signInThroughUi(page, acc.email, 'PasswordBaru1!');
    await expect(page).toHaveURL(/dashboard/);

    // Tautan yang sama tidak bisa dipakai dua kali.
    const again = await resetViaApi(request, token, 'PasswordLain2!');
    expect(again.status()).toBe(400);
    expect(await errorCode(again)).toBe('INVALID_TOKEN');
  });

  test('TC-31-03 Email tidak terdaftar mendapat jawaban yang sama dan tidak ada email terkirim', async ({ page, request }) => {
    const ghost = `ghost${Date.now()}@qms-test.local`;
    const res = await requestReset(request, ghost);
    expect(res.status()).toBe(202);

    await page.goto(`${WEB_URL}/forgot-password`);
    await field(page, 'EMAIL').fill(ghost);
    await action(page, 'Kirim').click();
    await expect(action(page, 'Cek Email Anda')).toBeVisible();

    await expectNoMail(request, ghost, 2500);
  });

  test('TC-31-04 Sesi lama dicabut setelah password diganti, token tanpa riwayat ganti tetap sah', async ({ request }) => {
    const acc = await registerAccount(request, 'revoke');
    const me = () => request.get(`${API_URL}/api/v1/users/me`, { headers: auth(acc.token) });
    expect((await me()).status()).toBe(200);

    await requestReset(request, acc.email);
    const token = await tokenFromMail(request, acc.email);
    // iat berdetik: tunggu supaya sesi lama pasti terbit lebih dulu dari penggantian password.
    await pause(1100);
    expect((await resetViaApi(request, token, 'PasswordBaru1!')).status()).toBe(200);

    expect((await me()).status()).toBe(401);
    const login = await request.post(`${API_URL}/api/v1/users/login`, { data: { identifier: acc.email, password: 'PasswordBaru1!' } });
    const fresh = ((await login.json()) as { data: { token: string } }).data.token;
    expect((await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(fresh) })).status()).toBe(200);
  });

  test('TC-31-05 Password pendek ditolak tanpa menghabiskan token', async ({ page, request }) => {
    const acc = await registerAccount(request, 'weak');
    await requestReset(request, acc.email);
    const token = await tokenFromMail(request, acc.email);

    const weak = await resetViaApi(request, token, 'pendek');
    expect(weak.status()).toBe(400);
    expect(await errorCode(weak)).toBe('WEAK_PASSWORD');

    // Di UI: password pendek tidak dikirim, tautan tetap bisa dipakai.
    await page.goto(`${WEB_URL}/reset-password?token=${encodeURIComponent(token)}`);
    await field(page, 'PASSWORD BARU').fill('pendek');
    await field(page, 'KONFIRMASI PASSWORD').fill('pendek');
    await action(page, 'Simpan Password').click();
    await expect(action(page, 'Password minimal 8 karakter')).toBeVisible();
    await expect(field(page, 'PASSWORD BARU')).toBeVisible();

    expect((await resetViaApi(request, token, 'PasswordBaru1!')).status()).toBe(200);
  });

  test('TC-31-06 Permintaan baru membatalkan tautan lama', async ({ request }) => {
    const acc = await registerAccount(request, 'twice');
    await requestReset(request, acc.email);
    const first = await tokenFromMail(request, acc.email);
    await requestReset(request, acc.email);
    // Email kedua: tunggu sampai ada dua email, lalu ambil yang berbeda dari yang pertama.
    let second = first;
    for (let i = 0; i < 30 && second === first; i++) {
      await pause(500);
      second = await tokenFromMail(request, acc.email);
    }
    expect(second).not.toBe(first);

    const old = await resetViaApi(request, first, 'PasswordBaru1!');
    expect(old.status()).toBe(400);
    expect((await resetViaApi(request, second, 'PasswordBaru1!')).status()).toBe(200);
  });

  test('TC-31-07 Halaman reset tanpa token atau dengan token sembarang menampilkan tautan tidak berlaku', async ({ page }) => {
    await page.goto(`${WEB_URL}/reset-password`);
    await expect(action(page, 'Tautan Tidak Berlaku')).toBeVisible();

    await page.goto(`${WEB_URL}/reset-password?token=bukan-token-asli`);
    await field(page, 'PASSWORD BARU').fill('PasswordBaru1!');
    await field(page, 'KONFIRMASI PASSWORD').fill('PasswordBaru1!');
    await action(page, 'Simpan Password').click();
    await expect(action(page, 'Tautan Tidak Berlaku')).toBeVisible();
    await expect(action(page, 'Minta Tautan Baru')).toBeVisible();
  });

  test('TC-31-08 Tombol di profil mengirim tautan ke email akun yang sedang login', async ({ page, request }) => {
    const acc = await registerAccount(request, 'prof');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await action(page, 'Minta Ganti Password').click();
    await expect(action(page, `Tautan untuk membuat password baru sudah dikirim ke ${acc.email}`)).toBeVisible();

    const token = await tokenFromMail(request, acc.email);
    expect(token.length).toBeGreaterThan(30);
  });

  test('TC-31-09 Halaman reset tidak membocorkan token lewat Referer dan tidak di-cache', async ({ request }) => {
    const res = await request.get(`${API_URL}/reset-password?token=abc`);
    // Lewat nginx (8080). Bila API_URL menunjuk backend langsung, header ini tidak ada: lewati.
    test.skip(!res.headers()['referrer-policy'], 'Tidak lewat nginx.');
    expect(res.headers()['referrer-policy']).toBe('no-referrer');
    expect(res.headers()['cache-control']).toContain('no-store');
  });

  test('TC-31-10 Permintaan dengan format email salah atau tanpa badan ditolak 400', async ({ request }) => {
    // Jeda antar permintaan supaya tidak kena batas laju nginx (1/detik, burst 5) yang bukan yang diuji di sini.
    const bad = async (path: string, data: object) => {
      await pause(1100);
      return request.post(`${API_URL}/api/v1/users/${path}`, { data });
    };
    for (const data of [{}, { email: 'bukan-email' }, { email: '<b>x</b>@a.com' }]) {
      expect((await bad('forgot-password', data)).status(), JSON.stringify(data)).toBe(400);
    }
    for (const data of [{}, { token: 'x' }, { password: 'PasswordBaru1!' }]) {
      expect((await bad('reset-password', data)).status(), JSON.stringify(data)).toBe(400);
    }
  });

  test('TC-31-11 Batas laju nginx: permintaan beruntun ke forgot-password akhirnya dijawab 429', async ({ request }) => {
    // Lewat nginx saja; bila API_URL menunjuk backend langsung tidak ada pembatas.
    await pause(6500); // kosongkan burst dari tes sebelumnya
    const probe = await request.get(`${API_URL}/reset-password`);
    test.skip(!probe.headers()['referrer-policy'], 'Tidak lewat nginx.');
    const codes: number[] = [];
    for (let i = 0; i < 12; i++) {
      codes.push((await request.post(`${API_URL}/api/v1/users/forgot-password`, { data: { email: 'rate@qms-test.local' } })).status());
    }
    expect(codes).toContain(429);
    expect(codes[0]).toBe(202);
  });
});
