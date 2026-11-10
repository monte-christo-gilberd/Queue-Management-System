import { expect, test, type BrowserContext, type Page } from '@playwright/test';

import { action, API_URL, AUTH_STORAGE_KEY, field, registerAccount, WEB_URL } from './support';

/**
 * TS-34 Login dengan Google (TC-09-12: 4.17, US-20).
 *
 * OAuth Google asli tidak bisa diotomasi (layar akun Google, captcha, dan kunci publik Google), jadi:
 *   - popup Google dicegat dan dijawab seperti Google mengalihkan balik ke aplikasi (dengan `state` yang sah);
 *   - POST /users/google dan /users/google/link disetubkan agar hasilnya bisa dikendalikan. Untuk sesi yang sah,
 *     tes mendaftarkan akun sungguhan lewat API dan mengembalikan tokennya dari stub.
 * Yang diuji di sini adalah sisi frontend: tombol, popup, pengiriman ID token, layar konfirmasi password, galat, dan
 * navigasi. Verifikasi ID token di server dan alur penautan dengan Google palsu diuji di tes Go (`auth-service`) dan
 * skrip end-to-end terhadap stack sungguhan. TC-34-10 dan TC-34-11 memanggil backend sungguhan tanpa stub.
 *
 * Prasyarat: aplikasi dibuka lewat `http://localhost` atau https (WebCrypto hanya ada di secure origin) dan dibangun
 * dengan EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID. Tanpa itu tombol Google tersembunyi dan tes di-skip.
 */

/** ID token palsu yang "dikembalikan Google". Hanya dikenali stub, bukan server. */
const STUB_ID_TOKEN = 'stub-google-id-token';
const LINK_TOKEN = 'stub-link-token';

type Json = Record<string, unknown>;

/** Panggilan ke endpoint Google yang disetubkan, untuk diperiksa tes. */
type Calls = { login: Json[]; link: Json[] };

/** Stub balasan untuk satu endpoint: status dan isi JSON. */
type Reply = { status: number; body: Json };

const apiError = (status: number, code: string, extra: Json = {}): Reply => ({
  status,
  body: { error: { code, message: code, ...extra } },
});

/**
 * Mencegat layar masuk Google. `success`: Google mengalihkan balik dengan id_token; `denied`: dengan error
 * access_denied; `hold`: halaman kosong yang tetap terbuka (pengguna belum memilih akun).
 */
async function stubGooglePopup(context: BrowserContext, mode: 'success' | 'denied' | 'hold') {
  await context.route(/accounts\.google\.com\/o\/oauth2\/v2\/auth/, async (route) => {
    if (mode === 'hold') {
      await route.fulfill({ status: 200, contentType: 'text/html', body: '<html><body>Google (stub)</body></html>' });
      return;
    }
    const url = new URL(route.request().url());
    const redirect = url.searchParams.get('redirect_uri') ?? '';
    const state = url.searchParams.get('state') ?? '';
    const fragment = mode === 'success' ? `id_token=${STUB_ID_TOKEN}` : 'error=access_denied';
    await route.fulfill({ status: 302, headers: { location: `${redirect}#${fragment}&state=${state}` } });
  });
}

/**
 * Menyetubkan POST /users/google dan /users/google/link. `login` dan `link` boleh fungsi agar balasan bisa berubah
 * per panggilan (mis. password salah lalu benar). Mengembalikan catatan panggilan.
 */
async function stubGoogleApi(
  page: Page,
  replies: { login?: Reply | ((n: number) => Reply); link?: Reply | ((n: number) => Reply) },
): Promise<Calls> {
  const calls: Calls = { login: [], link: [] };
  const answer = (r: Reply | ((n: number) => Reply), n: number) => (typeof r === 'function' ? r(n) : r);

  await page.route(/\/api\/v1\/users\/google$/, async (route) => {
    calls.login.push(route.request().postDataJSON() as Json);
    const reply = answer(replies.login ?? apiError(500, 'INTERNAL_ERROR'), calls.login.length);
    await route.fulfill({ status: reply.status, contentType: 'application/json', body: JSON.stringify(reply.body) });
  });
  await page.route(/\/api\/v1\/users\/google\/link$/, async (route) => {
    calls.link.push(route.request().postDataJSON() as Json);
    const reply = answer(replies.link ?? apiError(500, 'INTERNAL_ERROR'), calls.link.length);
    await route.fulfill({ status: reply.status, contentType: 'application/json', body: JSON.stringify(reply.body) });
  });
  return calls;
}

/** Token sesi sah milik akun baru (dikembalikan stub sebagai "token QMS" hasil login Google). */
async function realToken(request: Parameters<typeof registerAccount>[0]): Promise<string> {
  return (await registerAccount(request, 'goog', { verify: false })).token;
}

/** Token yang tersimpan di penyimpanan browser (zustand persist: {state: {token}}), atau null. */
async function storedToken(page: Page): Promise<string | null> {
  return page.evaluate((key) => {
    const raw = window.localStorage.getItem(key);
    if (!raw) return null;
    try {
      return (JSON.parse(raw) as { state?: { token?: string | null } }).state?.token ?? null;
    } catch {
      return null;
    }
  }, AUTH_STORAGE_KEY);
}

const GOOGLE_BUTTON = 'Login with Google';

/** Membuka halaman login dan menunggu tombol Google. Tes di-skip bila Google belum dikonfigurasi di bundle. */
async function openLogin(page: Page, testInfo: { skip: (cond: boolean, reason: string) => void }) {
  await page.goto(`${WEB_URL}/`);
  const visible = await action(page, GOOGLE_BUTTON)
    .waitFor({ state: 'visible', timeout: 20_000 })
    .then(() => true)
    .catch(() => false);
  testInfo.skip(
    !visible,
    'Tombol Google tidak tampil: isi EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID, bersihkan cache Metro, dan buka lewat http://localhost (bukan alamat IP).',
  );
}

/** Toast (role alert) yang memuat teks. */
const toast = (page: Page, text: string | RegExp) => page.getByRole('alert').filter({ hasText: text });

/** Masuk ke layar konfirmasi password: login Google dijawab 409 GOOGLE_LINK_REQUIRED. */
async function reachLinkScreen(page: Page, calls: { link: Reply | ((n: number) => Reply) }): Promise<Calls> {
  await stubGooglePopup(page.context(), 'success');
  const recorded = await stubGoogleApi(page, {
    login: apiError(409, 'GOOGLE_LINK_REQUIRED', { link_token: LINK_TOKEN }),
    link: calls.link,
  });
  await action(page, GOOGLE_BUTTON).click();
  await expect(action(page, 'Tautkan Akun Google')).toBeVisible();
  return recorded;
}

test.describe('TS-34 Login dengan Google', () => {
  test('TC-34-01 Tombol Google tampil di halaman login dan daftar', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    await expect(action(page, GOOGLE_BUTTON)).toBeVisible();

    await page.goto(`${WEB_URL}/register`);

    await expect(action(page, 'Daftar dengan Google')).toBeVisible();
  });

  test('TC-34-02 Login Google berhasil: ID token dikirim ke server dan sesi tersimpan', async ({ page, request }, testInfo) => {
    await openLogin(page, testInfo);
    const token = await realToken(request);
    await stubGooglePopup(page.context(), 'success');
    const calls = await stubGoogleApi(page, { login: { status: 200, body: { data: { token } } } });

    await action(page, GOOGLE_BUTTON).click();

    await expect(page).toHaveURL(/\/dashboard/);
    expect(calls.login).toEqual([{ id_token: STUB_ID_TOKEN }]);
    expect(await storedToken(page)).toBe(token);
  });

  test('TC-34-03 Email sudah terdaftar: diminta konfirmasi password, tanpa sesi sebelum itu', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);

    await reachLinkScreen(page, { link: apiError(401, 'UNAUTHORIZED') });

    await expect(field(page, 'PASSWORD')).toBeVisible();
    expect(await storedToken(page)).toBeNull();
  });

  test('TC-34-04 Password salah ditolak dengan pesan, benar menautkan dan masuk', async ({ page, request }, testInfo) => {
    await openLogin(page, testInfo);
    const token = await realToken(request);
    const calls = await reachLinkScreen(page, {
      link: (n) => (n === 1 ? apiError(401, 'UNAUTHORIZED') : { status: 200, body: { data: { token } } }),
    });

    await field(page, 'PASSWORD').fill('salah');
    await action(page, 'Tautkan').click();
    await expect(toast(page, 'Password salah')).toBeVisible();
    await expect(page).toHaveURL(/link-google/);
    expect(await storedToken(page)).toBeNull();

    await field(page, 'PASSWORD').fill('benar-benar');
    await action(page, 'Tautkan').click();

    await expect(page).toHaveURL(/\/dashboard/);
    expect(calls.link).toEqual([
      { link_token: LINK_TOKEN, password: 'salah' },
      { link_token: LINK_TOKEN, password: 'benar-benar' },
    ]);
    expect(await storedToken(page)).toBe(token);
  });

  test('TC-34-05 Password kosong ditolak di layar tanpa memanggil server', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    const calls = await reachLinkScreen(page, { link: apiError(401, 'UNAUTHORIZED') });

    await action(page, 'Tautkan').click();

    await expect(toast(page, 'Password wajib diisi')).toBeVisible();
    expect(calls.link).toHaveLength(0);
  });

  test('TC-34-06 Batal kembali ke login dan membuang link token', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    await reachLinkScreen(page, { link: apiError(401, 'UNAUTHORIZED') });

    await action(page, 'Batal').click();

    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
    // Menuju layar penautan lagi lewat navigasi di sisi klien (history + popstate), TANPA memuat ulang halaman. Muat ulang
    // penuh (page.goto) menghapus seluruh memori sehingga tidak membuktikan apa pun tentang Batal (itu TC-34-07).
    // Bila link token tidak dibuang saat Batal, layar penautan akan tampil lagi di sini.
    await page.evaluate(() => {
      window.history.pushState({}, '', '/link-google');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
    await expect(action(page, 'Tautkan Akun Google')).toHaveCount(0);
    expect(new URL(page.url()).pathname).not.toContain('link-google');
  });

  test('TC-34-07 Refresh di layar penautan kembali ke login (link token hanya di memori)', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    await reachLinkScreen(page, { link: apiError(401, 'UNAUTHORIZED') });

    await page.reload();

    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
    await expect(action(page, 'Tautkan Akun Google')).toHaveCount(0);
  });

  test('TC-34-08 Link token habis: pesan jelas lalu kembali ke login', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    await reachLinkScreen(page, { link: apiError(400, 'INVALID_TOKEN') });

    await field(page, 'PASSWORD').fill('apa-saja');
    await action(page, 'Tautkan').click();

    await expect(toast(page, /Sesi penautan habis/)).toBeVisible();
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
  });

  const ERRORS: Array<[title: string, reply: Reply, message: RegExp]> = [
    ['ID token ditolak (401)', apiError(401, 'UNAUTHORIZED'), /Login dengan Google gagal/],
    ['email belum diverifikasi Google (403)', apiError(403, 'GOOGLE_EMAIL_UNVERIFIED'), /belum diverifikasi oleh Google/],
    ['Google tidak terjangkau (503)', apiError(503, 'GOOGLE_UNAVAILABLE'), /Google sedang tidak tersedia/],
    ['auth-service mati (503 dari nginx)', apiError(503, 'AUTH_UNAVAILABLE'), /Login sedang tidak tersedia/],
    ['akun sudah tertaut ke Google lain (409)', apiError(409, 'CONFLICT'), /tertaut ke akun Google yang lain/],
  ];
  for (const [title, reply, message] of ERRORS) {
    test(`TC-34-09 Galat server tampil sebagai toast tanpa sesi: ${title}`, async ({ page }, testInfo) => {
      await openLogin(page, testInfo);
      await stubGooglePopup(page.context(), 'success');
      await stubGoogleApi(page, { login: reply });

      await action(page, GOOGLE_BUTTON).click();

      await expect(toast(page, message)).toBeVisible();
      await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
      expect(await storedToken(page)).toBeNull();
    });
  }

  test('TC-34-10 Popup ditutup tanpa memilih akun: diam, tanpa panggilan server dan tanpa toast', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    await stubGooglePopup(page.context(), 'hold');
    const calls = await stubGoogleApi(page, { login: apiError(500, 'INTERNAL_ERROR') });

    const popupPromise = page.waitForEvent('popup');
    await action(page, GOOGLE_BUTTON).click();
    const popup = await popupPromise;
    await popup.close();
    await page.waitForTimeout(1500);

    expect(calls.login).toHaveLength(0);
    await expect(page.getByRole('alert')).toHaveCount(0);
    await expect(field(page, 'EMAIL ATAU USERNAME')).toBeVisible();
  });

  test('TC-34-11 Google menolak (access_denied): toast, tanpa panggilan server', async ({ page }, testInfo) => {
    await openLogin(page, testInfo);
    await stubGooglePopup(page.context(), 'denied');
    const calls = await stubGoogleApi(page, { login: apiError(500, 'INTERNAL_ERROR') });

    await action(page, GOOGLE_BUTTON).click();

    await expect(toast(page, /Login dengan Google gagal/)).toBeVisible();
    expect(calls.login).toHaveLength(0);
  });
});

// Dua tes di bawah memanggil backend sungguhan (tanpa stub). Dilewati bila Google tidak dinyalakan di stack
// (GOOGLE_CLIENT_IDS kosong: rute 404).
test.describe('TS-34 Kontrak API login Google (backend sungguhan)', () => {
  test.beforeEach(async ({ request }, testInfo) => {
    const probe = await request.post(`${API_URL}/api/v1/users/google`, { data: { id_token: 'x' } });
    testInfo.skip(probe.status() === 404, 'Login Google tidak dinyalakan di stack ini (GOOGLE_CLIENT_IDS kosong).');
  });

  test('TC-34-12 ID token palsu atau body salah ditolak dan tidak membuat akun', async ({ request }) => {
    const garbage = await request.post(`${API_URL}/api/v1/users/google`, { data: { id_token: 'abc.def.ghi' } });
    expect(garbage.status()).toBe(401);
    expect(((await garbage.json()) as { error: { code: string } }).error.code).toBe('UNAUTHORIZED');
    expect(garbage.headers()['cache-control']).toBe('no-store');

    const empty = await request.post(`${API_URL}/api/v1/users/google`, { data: {} });
    expect(empty.status()).toBe(400);

    // alg=none tanpa tanda tangan juga ditolak
    const none = `${Buffer.from('{"alg":"none","typ":"JWT"}').toString('base64url')}.${Buffer.from(
      '{"iss":"https://accounts.google.com","sub":"1","email":"a@b.c","email_verified":true}',
    ).toString('base64url')}.`;
    expect((await request.post(`${API_URL}/api/v1/users/google`, { data: { id_token: none } })).status()).toBe(401);
  });

  test('TC-34-13 Link token palsu ditolak dan tidak membuka sesi', async ({ request }) => {
    const bad = await request.post(`${API_URL}/api/v1/users/google/link`, {
      data: { link_token: 'sampah', password: 'rahasia123' },
    });
    expect(bad.status()).toBe(400);
    expect(((await bad.json()) as { error: { code: string } }).error.code).toBe('INVALID_TOKEN');

    const missing = await request.post(`${API_URL}/api/v1/users/google/link`, { data: {} });
    expect(missing.status()).toBe(400);

    // JWT berbentuk link token (klaim `purpose`) tetapi bertanda tangan sembarang: ditolak sebagai sesi. Catatan: ini
    // membuktikan penolakan tanda tangan saja. Bahwa link token ASLI (bertanda tangan sah) tidak bisa menjadi sesi
    // dibuktikan di tes Go (auth-service, backend, payment-service) dan skrip end-to-end dengan Google palsu,
    // karena link token asli hanya bisa didapat dari alur Google yang tidak bisa diotomasi di sini.
    const fakeLink = `${Buffer.from('{"alg":"RS256","typ":"JWT"}').toString('base64url')}.${Buffer.from(
      JSON.stringify({ sub: '1', purpose: 'google_link', gid: 'g', exp: Math.floor(Date.now() / 1000) + 300 }),
    ).toString('base64url')}.c2lnbmF0dXJl`;
    const me = await request.get(`${API_URL}/api/v1/users/me`, { headers: { Authorization: `Bearer ${fakeLink}` } });
    expect(me.status()).toBe(401);
  });
});
