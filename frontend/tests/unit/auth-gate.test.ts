/// <reference types="node" />
// Sumber: product-overview.md — US-14 (wajib login sebelum bergabung, kembali ke halaman join),
// TC-01-09 (halaman terlindungi → login → kembali), TC-01-12 (next ke situs lain diabaikan).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { decideAuthGate, HOME_PATH, LOGIN_PATH } from '@/lib/auth-gate';

test('selama storage belum selesai dibaca tidak ada keputusan (cegah redirect sekilas)', () => {
  assert.deepEqual(decideAuthGate({ mode: 'required', hydrated: false, token: null, path: '/queues' }), { kind: 'wait' });
  assert.deepEqual(decideAuthGate({ mode: 'guest', hydrated: false, token: 'abc', next: '/join/1' }), { kind: 'wait' });
});

test('halaman terlindungi tanpa login diarahkan ke login dengan ?next= halaman asal', () => {
  const decision = decideAuthGate({ mode: 'required', hydrated: true, token: null, path: '/queues' });
  assert.deepEqual(decision, { kind: 'redirect', to: '/?next=%2Fqueues' });
});

test('tautan join tanpa login meminta login dan menyimpan /join/<id> sebagai tujuan (US-14 AC1)', () => {
  const decision = decideAuthGate({ mode: 'required', hydrated: true, token: null, path: '/join/12' });
  assert.deepEqual(decision, { kind: 'redirect', to: '/?next=%2Fjoin%2F12' });
});

test('halaman terlindungi dengan token boleh dirender', () => {
  assert.deepEqual(decideAuthGate({ mode: 'required', hydrated: true, token: 'tok', path: '/queues' }), { kind: 'render' });
});

test('token kosong dihitung belum login', () => {
  const decision = decideAuthGate({ mode: 'required', hydrated: true, token: '', path: '/dashboard' });
  assert.deepEqual(decision, { kind: 'redirect', to: '/?next=%2Fdashboard' });
});

test('dari halaman login (/) tidak menambahkan ?next= (sudah di login)', () => {
  assert.deepEqual(decideAuthGate({ mode: 'required', hydrated: true, token: null, path: LOGIN_PATH }), {
    kind: 'redirect',
    to: LOGIN_PATH,
  });
  assert.deepEqual(decideAuthGate({ mode: 'required', hydrated: true, token: null }), {
    kind: 'redirect',
    to: LOGIN_PATH,
  });
});

test('parameter next dari halaman login/register dikembalikan dengan benar setelah login (round trip)', () => {
  const original = '/join/42';
  const toLogin = decideAuthGate({ mode: 'required', hydrated: true, token: null, path: original });
  assert.equal(toLogin.kind, 'redirect');
  const encoded = new URL(toLogin.kind === 'redirect' ? toLogin.to : '', 'http://x').searchParams.get('next');
  assert.equal(encoded, original);
});

test('halaman login tanpa token dirender', () => {
  assert.deepEqual(decideAuthGate({ mode: 'guest', hydrated: true, token: null, next: '/join/1' }), { kind: 'render' });
});

test('pengguna yang sudah login dari halaman login dilanjutkan ke next yang aman (US-14 kembali ke join)', () => {
  assert.deepEqual(decideAuthGate({ mode: 'guest', hydrated: true, token: 'tok', next: '/join/12' }), {
    kind: 'redirect',
    to: '/join/12',
  });
});

test('pengguna yang sudah login tanpa next diarahkan ke dashboard', () => {
  assert.deepEqual(decideAuthGate({ mode: 'guest', hydrated: true, token: 'tok' }), { kind: 'redirect', to: HOME_PATH });
  assert.deepEqual(decideAuthGate({ mode: 'guest', hydrated: true, token: 'tok', next: '/' }), {
    kind: 'redirect',
    to: HOME_PATH,
  });
});

test('next ke situs lain tidak dipakai setelah login (TC-01-12)', () => {
  for (const next of ['https://evil.example', '//evil.example', '/\\evil.example']) {
    assert.deepEqual(decideAuthGate({ mode: 'guest', hydrated: true, token: 'tok', next }), {
      kind: 'redirect',
      to: HOME_PATH,
    });
  }
});
