/// <reference types="node" />
// Logika notifikasi Android (FCM) tanpa modul native: expo-notifications, AsyncStorage, dan apiFetch dipalsukan.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { createNativePush, notificationUrl, STORAGE_KEY, type NativePushDeps } from '@/lib/push-native-core';

/** Call: satu request ke backend yang dicatat api palsu. */
type Call = { path: string; body: unknown };

/** fake: dependensi palsu dengan izin, token, dan storage yang bisa diatur; mencatat request ke backend. */
function fake(opts: { available?: boolean; granted?: boolean; grantOnAsk?: boolean; token?: string; stored?: string; apiFails?: boolean } = {}) {
  const store = new Map<string, string>();
  if (opts.stored) store.set(STORAGE_KEY, opts.stored);
  const calls: Call[] = [];
  // state: izin saat ini, berapa kali dialog izin ditampilkan, berapa kali kanal dibuat, dan token FCM perangkat.
  const state = { granted: opts.granted ?? false, asked: 0, channels: 0, token: opts.token ?? 'tok-1' };
  const deps: NativePushDeps = {
    available: opts.available ?? true,
    ensureChannel: async () => {
      state.channels++;
    },
    permissionGranted: async () => state.granted,
    // Dialog izin: hasilnya ditentukan opts.grantOnAsk (pengguna menekan Izinkan atau Jangan izinkan).
    requestPermission: async () => {
      state.asked++;
      state.granted = opts.grantOnAsk ?? false;
      return state.granted;
    },
    deviceToken: async () => state.token,
    // api palsu: mencatat request, atau melempar bila opts.apiFails (mis. offline).
    api: async (path, init) => {
      if (opts.apiFails) throw new Error('offline');
      calls.push({ path, body: init.body });
      return undefined;
    },
    // storage di memori, pengganti AsyncStorage.
    storage: {
      getItem: (k) => store.get(k) ?? null,
      setItem: (k, v) => void store.set(k, v),
      removeItem: (k) => void store.delete(k),
    },
  };
  return { push: createNativePush(deps), calls, store, state };
}

test('enable: izin diberikan -> kanal dibuat, token didaftarkan sebagai android, tanda lokal disimpan', async () => {
  const f = fake({ grantOnAsk: true });
  assert.equal(await f.push.enable(), true);
  assert.equal(f.state.channels, 1);
  assert.deepEqual(f.calls, [{ path: '/api/v1/push/fcm/register', body: { token: 'tok-1', platform: 'android' } }]);
  assert.equal(f.store.get(STORAGE_KEY), 'tok-1');
});

test('enable: izin sudah ada -> tidak meminta izin lagi', async () => {
  const f = fake({ granted: true });
  assert.equal(await f.push.enable(), true);
  assert.equal(f.state.asked, 0);
});

test('enable: izin ditolak -> false, tidak ada token terdaftar', async () => {
  const f = fake({ grantOnAsk: false });
  assert.equal(await f.push.enable(), false);
  assert.equal(f.calls.length, 0);
  assert.equal(f.store.has(STORAGE_KEY), false);
});

test('Expo Go/iOS (tidak tersedia): tidak didukung dan tidak memanggil apa pun', async () => {
  const f = fake({ available: false, granted: true, stored: 'tok-1' });
  assert.equal(f.push.supported(), false);
  assert.equal(await f.push.enable(), false);
  assert.equal(await f.push.sync(), false);
  await f.push.ensureChannel();
  await f.push.tokenChanged('tok-2');
  assert.equal(f.calls.length, 0);
  assert.equal(f.state.channels, 0);
});

test('disable: lepas token tersimpan lalu hapus tanda lokal; tanpa token tidak melakukan apa-apa', async () => {
  const f = fake({ stored: 'tok-1' });
  await f.push.disable();
  assert.deepEqual(f.calls, [{ path: '/api/v1/push/fcm/unregister', body: { token: 'tok-1' } }]);
  assert.equal(f.store.has(STORAGE_KEY), false);
  await f.push.disable();
  assert.equal(f.calls.length, 1);
});

test('disable: request gagal -> melempar dan tanda lokal tetap ada', async () => {
  const f = fake({ stored: 'tok-1', apiFails: true });
  await assert.rejects(f.push.disable());
  assert.equal(f.store.get(STORAGE_KEY), 'tok-1');
});

test('sync: belum aktif -> false tanpa request', async () => {
  const f = fake({ granted: true });
  assert.equal(await f.push.sync(), false);
  assert.equal(f.calls.length, 0);
});

test('sync: aktif dan izin ada -> token didaftarkan ulang ke akun yang login', async () => {
  const f = fake({ granted: true, stored: 'tok-1' });
  assert.equal(await f.push.sync(), true);
  assert.deepEqual(f.calls, [{ path: '/api/v1/push/fcm/register', body: { token: 'tok-1', platform: 'android' } }]);
});

test('sync: token berganti -> token lama dilepas, token baru didaftarkan', async () => {
  const f = fake({ granted: true, stored: 'old', token: 'new' });
  assert.equal(await f.push.sync(), true);
  assert.deepEqual(
    f.calls.map((c) => c.path),
    ['/api/v1/push/fcm/unregister', '/api/v1/push/fcm/register'],
  );
  assert.equal(f.store.get(STORAGE_KEY), 'new');
});

test('sync: izin dicabut dari Pengaturan -> token dilepas dan dianggap mati', async () => {
  const f = fake({ granted: false, stored: 'tok-1' });
  assert.equal(await f.push.sync(), false);
  assert.deepEqual(f.calls, [{ path: '/api/v1/push/fcm/unregister', body: { token: 'tok-1' } }]);
  assert.equal(f.store.has(STORAGE_KEY), false);
});

test('tokenChanged: hanya daftar ulang bila notifikasi aktif dan token benar-benar berubah', async () => {
  const off = fake();
  await off.push.tokenChanged('tok-2');
  assert.equal(off.calls.length, 0);

  const same = fake({ stored: 'tok-1' });
  await same.push.tokenChanged('tok-1');
  assert.equal(same.calls.length, 0);

  const on = fake({ stored: 'tok-1' });
  await on.push.tokenChanged('tok-2');
  assert.deepEqual(
    on.calls.map((c) => c.path),
    ['/api/v1/push/fcm/unregister', '/api/v1/push/fcm/register'],
  );
  assert.equal(on.store.get(STORAGE_KEY), 'tok-2');
});

test('releaseForLogout: melepas token dan tidak pernah melempar', async () => {
  const ok = fake({ stored: 'tok-1' });
  await ok.push.releaseForLogout();
  assert.deepEqual(ok.calls, [{ path: '/api/v1/push/fcm/unregister', body: { token: 'tok-1' } }]);
  assert.equal(ok.store.has(STORAGE_KEY), false);

  const failing = fake({ stored: 'tok-1', apiFails: true });
  await assert.doesNotReject(failing.push.releaseForLogout());
});

test('notificationUrl: hanya path internal yang diterima', () => {
  assert.equal(notificationUrl('/ticket/12'), '/ticket/12');
  assert.equal(notificationUrl(undefined, '/ticket/3'), '/ticket/3');
  assert.equal(notificationUrl('https://evil.example'), null);
  assert.equal(notificationUrl('//evil.example/x'), null);
  assert.equal(notificationUrl(42, null), null);
});
