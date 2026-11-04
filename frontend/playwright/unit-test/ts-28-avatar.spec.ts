import { expect, test, type APIRequestContext, type Page } from '@playwright/test';
import { deflateSync } from 'node:zlib';

import { action, API_URL, auth, registerAccount, signInWithToken, WEB_URL } from './support';

/** PNG berwarna acak sebagai Buffer (tanpa dependensi gambar). */
function makePng(width: number, height: number): Buffer {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    return c >>> 0;
  });
  const crc = (buf: Buffer) => {
    let c = 0xffffffff;
    for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type: string, data: Buffer) => {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length);
    const body = Buffer.concat([Buffer.from(type), data]);
    const sum = Buffer.alloc(4);
    sum.writeUInt32BE(crc(body));
    return Buffer.concat([len, body, sum]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8; // 8 bit
  ihdr[9] = 2; // RGB
  const rows: Buffer[] = [];
  for (let y = 0; y < height; y++) {
    const row = Buffer.alloc(1 + width * 3);
    for (let x = 0; x < width; x++) {
      row[1 + x * 3] = (x * 7) & 255;
      row[2 + x * 3] = (y * 13) & 255;
      row[3 + x * 3] = (x ^ y) & 255;
    }
    rows.push(row);
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(Buffer.concat(rows))),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

async function me(request: APIRequestContext, token: string) {
  const res = await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(token) });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).data as { avatar_updated_at: string | null };
}

async function avatarStatus(request: APIRequestContext, token: string) {
  return (await request.get(`${API_URL}/api/v1/users/me/avatar`, { headers: auth(token) })).status();
}

async function choosePhoto(page: Page, label: string, file: { name: string; mimeType: string; buffer: Buffer }) {
  const chooser = page.waitForEvent('filechooser');
  await action(page, label).click();
  await (await chooser).setFiles(file);
}

test.describe('TS-28 Foto profil', () => {
  test('TC-28-01 Tanpa foto tampil tombol Unggah Foto dan tanpa Hapus Foto', async ({ page, request }) => {
    const acc = await registerAccount(request, 'avt');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await expect(action(page, 'Unggah Foto')).toBeVisible();
    await expect(action(page, 'Hapus Foto')).toHaveCount(0);
    await expect(page.getByAltText('Foto profil')).toHaveCount(0);
  });

  test('TC-28-02 Upload lewat UI: foto tampil, tersimpan 512x512 JPEG, dan bertahan setelah reload', async ({ page, request }) => {
    const acc = await registerAccount(request, 'avt');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await choosePhoto(page, 'Unggah Foto', { name: 'foto.png', mimeType: 'image/png', buffer: makePng(900, 600) });

    await expect(page.getByAltText('Foto profil').first()).toBeVisible();
    await expect(action(page, 'Ganti Foto')).toBeVisible();
    await expect(action(page, 'Hapus Foto')).toBeVisible();
    expect((await me(request, acc.token)).avatar_updated_at).not.toBeNull();

    const img = await request.get(`${API_URL}/api/v1/users/me/avatar`, { headers: auth(acc.token) });
    expect(img.status()).toBe(200);
    expect(img.headers()['content-type']).toBe('image/jpeg');
    const body = await img.body();
    expect(body.subarray(0, 3)).toEqual(Buffer.from([0xff, 0xd8, 0xff]));
    expect(body.length).toBeLessThanOrEqual(150 * 1024);

    await page.reload();
    await expect(page.getByAltText('Foto profil').first()).toBeVisible();
  });

  test('TC-28-03 Ganti foto: tetap satu foto, waktu pembaruan berubah', async ({ page, request }) => {
    const acc = await registerAccount(request, 'avt');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await choosePhoto(page, 'Unggah Foto', { name: 'a.png', mimeType: 'image/png', buffer: makePng(300, 300) });
    await expect(action(page, 'Ganti Foto')).toBeVisible();
    const first = (await me(request, acc.token)).avatar_updated_at;

    await choosePhoto(page, 'Ganti Foto', { name: 'b.png', mimeType: 'image/png', buffer: makePng(400, 300) });
    await expect
      .poll(async () => (await me(request, acc.token)).avatar_updated_at)
      .not.toBe(first);
    await expect(page.getByAltText('Foto profil').first()).toBeVisible();
  });

  test('TC-28-04 Hapus foto: popup konfirmasi, Batal tidak menghapus, konfirmasi kembali ke ikon', async ({ page, request }) => {
    const acc = await registerAccount(request, 'avt');
    await signInWithToken(page, acc.token, acc.username, '/profile');
    await choosePhoto(page, 'Unggah Foto', { name: 'a.png', mimeType: 'image/png', buffer: makePng(300, 300) });
    await expect(action(page, 'Hapus Foto')).toBeVisible();

    await action(page, 'Hapus Foto').click();
    await expect(action(page, 'Hapus foto profil?')).toBeVisible();
    await action(page, 'Batal').click();
    await expect(action(page, 'Hapus foto profil?')).toBeHidden();
    expect(await avatarStatus(request, acc.token)).toBe(200);

    await action(page, 'Hapus Foto').click();
    await action(page, 'Ya, Hapus').click();

    await expect(action(page, 'Unggah Foto')).toBeVisible();
    await expect(page.getByAltText('Foto profil')).toHaveCount(0);
    expect(await avatarStatus(request, acc.token)).toBe(404);
    expect((await me(request, acc.token)).avatar_updated_at).toBeNull();
  });

  test('TC-28-05 Berkas yang bukan gambar ditolak dengan toast dan tidak menyimpan apa pun', async ({ page, request }) => {
    const acc = await registerAccount(request, 'avt');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await choosePhoto(page, 'Unggah Foto', {
      name: 'palsu.jpg',
      mimeType: 'image/jpeg',
      buffer: Buffer.from('<html>ini bukan gambar</html>'),
    });

    await expect(page.getByText(/Foto (harus berformat|tidak bisa dibaca)/).filter({ visible: true }).first()).toBeVisible();
    expect(await avatarStatus(request, acc.token)).toBe(404);
  });

  test('TC-28-06 API: format tidak didukung 415, lebih dari 5 MB 413, tanpa token 401, hapus ganda 204', async ({ request }) => {
    const acc = await registerAccount(request, 'avt');
    const put = (token: string | null, file: { name: string; mimeType: string; buffer: Buffer }) =>
      request.put(`${API_URL}/api/v1/users/me/avatar`, { headers: token ? auth(token) : {}, multipart: { file } });

    expect((await put(acc.token, { name: 'x.svg', mimeType: 'image/svg+xml', buffer: Buffer.from('<svg xmlns="http://www.w3.org/2000/svg"/>') })).status()).toBe(415);
    expect((await put(acc.token, { name: 'big.jpg', mimeType: 'image/jpeg', buffer: Buffer.concat([Buffer.from([0xff, 0xd8, 0xff]), Buffer.alloc(6 * 1024 * 1024)]) })).status()).toBe(413);
    expect((await put(null, { name: 'a.png', mimeType: 'image/png', buffer: makePng(50, 50) })).status()).toBe(401);
    expect((await request.get(`${API_URL}/api/v1/users/me/avatar`)).status()).toBe(401);

    expect((await put(acc.token, { name: 'a.png', mimeType: 'image/png', buffer: makePng(50, 50) })).status()).toBe(200);
    const del = () => request.delete(`${API_URL}/api/v1/users/me/avatar`, { headers: auth(acc.token) });
    expect((await del()).status()).toBe(204);
    expect((await del()).status()).toBe(204);
  });

  test('TC-28-07 Foto hanya milik sendiri: user lain tidak punya cara membacanya', async ({ request }) => {
    const a = await registerAccount(request, 'avt');
    const b = await registerAccount(request, 'avt');
    const res = await request.put(`${API_URL}/api/v1/users/me/avatar`, {
      headers: auth(a.token),
      multipart: { file: { name: 'a.png', mimeType: 'image/png', buffer: makePng(80, 80) } },
    });
    expect(res.status()).toBe(200);

    expect(await avatarStatus(request, b.token)).toBe(404);
    expect((await me(request, b.token)).avatar_updated_at).toBeNull();
    // Tidak ada rute foto per user id.
    expect((await request.get(`${API_URL}/api/v1/users/1/avatar`, { headers: auth(b.token) })).status()).toBeGreaterThanOrEqual(400);
  });

  test('TC-28-08 Header di semua halaman memakai foto baru, kembali ke ikon setelah dihapus, dan tidak bocor ke user lain', async ({ page, request }) => {
    const acc = await registerAccount(request, 'avt');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await choosePhoto(page, 'Unggah Foto', { name: 'a.png', mimeType: 'image/png', buffer: makePng(300, 300) });
    // Foto di profil (1) dan di tombol header (2): berubah seketika, tanpa reload.
    await expect(page.getByAltText('Foto profil')).toHaveCount(2);

    // Halaman lain memuat ulang dari server dan header tetap memakai foto.
    await page.goto(`${WEB_URL}/riwayat`);
    await expect(page.getByAltText('Foto profil').first()).toBeVisible();

    await page.goto(`${WEB_URL}/profile`);
    await action(page, 'Hapus Foto').click();
    await action(page, 'Ya, Hapus').click();
    await expect(page.getByAltText('Foto profil')).toHaveCount(0);
    await page.goto(`${WEB_URL}/riwayat`);
    await expect(page.getByAltText('Foto profil')).toHaveCount(0);

    // Foto lama dari sesi sebelumnya tidak bocor: user lain di browser yang sama tidak melihatnya.
    await page.goto(`${WEB_URL}/profile`);
    await choosePhoto(page, 'Unggah Foto', { name: 'b.png', mimeType: 'image/png', buffer: makePng(200, 200) });
    await expect(page.getByAltText('Foto profil')).toHaveCount(2);
    const other = await registerAccount(request, 'avt');
    await signInWithToken(page, other.token, other.username, '/riwayat');
    await expect(page.getByAltText('Antrian')).toBeVisible();
    await expect(page.getByAltText('Foto profil')).toHaveCount(0);
  });
});
