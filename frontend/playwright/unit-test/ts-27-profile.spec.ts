import { expect, test } from '@playwright/test';

import {
  action,
  API_URL,
  auth,
  field,
  registerAccount,
  signInWithToken,
  subscribedOwnerSession,
  uniqueName,
} from './support';

async function me(request: import('@playwright/test').APIRequestContext, token: string) {
  const res = await request.get(`${API_URL}/api/v1/users/me`, { headers: auth(token) });
  expect(res.ok()).toBeTruthy();
  return (await res.json()).data as { username: string; email: string; is_owner: boolean; subscription_expires_at: string | null };
}

test.describe('TS-27 Halaman profil', () => {
  test('TC-27-01 Customer melihat nama dan email, tanpa kartu langganan', async ({ page, request }) => {
    const acc = await registerAccount(request, 'prof');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await expect(field(page, 'Nama User')).toHaveValue(acc.username);
    await expect(field(page, 'Email User')).toHaveValue(acc.email);
    await expect(field(page, 'Email User')).not.toBeEditable();
    await expect(action(page, 'Status Subscription')).toHaveCount(0);
  });

  test('TC-27-01b Customer melihat ajakan Jadi Owner yang menuju halaman Subscription', async ({ page, request }) => {
    const acc = await registerAccount(request, 'prof');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await expect(action(page, 'Jadi Owner')).toBeVisible();
    await action(page, 'Jadi Owner & Bayar →').click();
    await expect(page).toHaveURL(/\/subscription$/);
    await expect(action(page, 'Belum Jadi Owner')).toBeVisible();
    await expect(action(page, 'JADI OWNER (BAYAR 1 BULAN)')).toBeVisible();
  });

  test('TC-27-01c Owner tidak melihat ajakan Jadi Owner', async ({ page, request }) => {
    const owner = await subscribedOwnerSession(request);
    test.skip(owner === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
    if (!owner) return;
    await signInWithToken(page, owner.token, owner.displayName, '/profile');

    await expect(action(page, 'Status Subscription')).toBeVisible();
    await expect(action(page, 'Jadi Owner')).toHaveCount(0);
  });

  test('TC-27-02 Mengubah nama tersimpan di server dan tampil di header', async ({ page, request }) => {
    const acc = await registerAccount(request, 'prof');
    await signInWithToken(page, acc.token, acc.username, '/profile');
    const next = uniqueName('baru');

    await field(page, 'Nama User').fill(`  ${next}  `);
    await action(page, 'Simpan Perubahan').click();

    await expect(action(page, 'Perubahan tersimpan')).toBeVisible();
    expect((await me(request, acc.token)).username).toBe(next);
    await expect(field(page, 'Nama User')).toHaveValue(next);
    // Header membaca displayName dari store; setelah reload nama baru tetap ada.
    await page.reload();
    await expect(field(page, 'Nama User')).toHaveValue(next);
  });

  test('TC-27-03 Nama kosong atau tidak berubah tidak bisa disimpan', async ({ page, request }) => {
    const acc = await registerAccount(request, 'prof');
    await signInWithToken(page, acc.token, acc.username, '/profile');
    await expect(field(page, 'Nama User')).toHaveValue(acc.username);

    const save = action(page, 'Simpan Perubahan');
    await save.click({ force: true });
    await expect(action(page, 'Perubahan tersimpan')).toHaveCount(0);

    await field(page, 'Nama User').fill('   ');
    await save.click({ force: true });
    await expect(action(page, 'Perubahan tersimpan')).toHaveCount(0);
    expect((await me(request, acc.token)).username).toBe(acc.username);
  });

  test('TC-27-04 Tombol ganti password tampil; alur lengkapnya diuji di ts-31 (item 9)', async ({ page, request }) => {
    const acc = await registerAccount(request, 'prof');
    await signInWithToken(page, acc.token, acc.username, '/profile');

    await expect(action(page, 'Minta Ganti Password')).toBeVisible();
  });

  test('TC-27-05 API: username tidak valid ditolak, tanpa token 401, email tidak ikut berubah', async ({ request }) => {
    const acc = await registerAccount(request, 'prof');
    const put = (token: string | null, username: string) =>
      request.put(`${API_URL}/api/v1/users/me`, { headers: token ? auth(token) : {}, data: { username } });

    expect((await put(acc.token, '   ')).status()).toBe(400);
    expect((await put(acc.token, 'a'.repeat(51))).status()).toBe(400);
    expect((await put(acc.token, '<b>x</b>')).status()).toBe(400);
    expect((await put(null, 'x')).status()).toBe(401);

    const res = await request.put(`${API_URL}/api/v1/users/me`, {
      headers: auth(acc.token),
      data: { username: 'ok-nama', email: 'lain@qms-test.local' },
    });
    expect(res.status()).toBe(200);
    const after = await me(request, acc.token);
    expect(after.email).toBe(acc.email);
    expect(after.username).toBe('ok-nama');
  });

  test('TC-27-06 Owner berlangganan melihat status dan masa langganan', async ({ page, request }) => {
    const owner = await subscribedOwnerSession(request);
    test.skip(owner === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
    if (!owner) return;
    const profile = await me(request, owner.token);
    expect(profile.is_owner).toBe(true);
    expect(profile.subscription_expires_at).not.toBeNull();

    await signInWithToken(page, owner.token, owner.displayName, '/profile');
    await expect(action(page, 'Status Subscription')).toBeVisible();
    await expect(action(page, 'Aktif')).toBeVisible();
    await expect(page.getByText(/^Aktif hingga /).filter({ visible: true })).toBeVisible();
  });

  test('TC-27-07 Owner aktif tetap bisa membuka halaman Subscription untuk memperpanjang', async ({ page, request }) => {
    const owner = await subscribedOwnerSession(request);
    test.skip(owner === null, 'Set QMS_OWNER_EMAIL dan QMS_OWNER_PASSWORD (owner berlangganan).');
    if (!owner) return;
    await signInWithToken(page, owner.token, owner.displayName, '/profile');

    await action(page, 'Perpanjang Langganan →').click();
    await expect(page).toHaveURL(/\/subscription$/);
    await expect(action(page, 'Langganan Aktif')).toBeVisible();
    await expect(action(page, 'BAYAR 1 BULAN')).toBeVisible();
  });
});
