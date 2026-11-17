/// <reference types="node" />
// Sumber: product-overview.md, TC-01-09 (kembali ke halaman asal setelah login) dan
// TC-01-12 / keamanan open redirect (`?next=` tidak boleh membawa pengguna ke situs lain).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { DEFAULT_AFTER_LOGIN, sanitizeNext } from '@/lib/safe-next';

test('path internal dikembalikan apa adanya (kembali ke halaman asal, TC-01-09)', () => {
  assert.equal(sanitizeNext('/queues'), '/queues');
  assert.equal(sanitizeNext('/ticket/12'), '/ticket/12');
  assert.equal(sanitizeNext('/queues/7?tab=tickets'), '/queues/7?tab=tickets');
  assert.equal(sanitizeNext('/join?t=v1.abc.def'), '/join?t=v1.abc.def');
  assert.equal(sanitizeNext('/join?t=v1.abc.def&x=1'), '/join?t=v1.abc.def&x=1');
});

test('URL absolut ke situs lain ditolak (open redirect, TC-01-12)', () => {
  assert.equal(sanitizeNext('https://evil.example'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('http://evil.example/dashboard'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('javascript:alert(1)'), DEFAULT_AFTER_LOGIN);
});

test('protocol-relative //host ditolak', () => {
  assert.equal(sanitizeNext('//evil.example'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('//evil.example/join/1'), DEFAULT_AFTER_LOGIN);
});

test('backslash dan karakter kontrol ditolak (bypass browser: /\\host, tab/newline)', () => {
  assert.equal(sanitizeNext('/\\evil.example'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('\\\\evil.example'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('/\t/evil.example'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('/dashboard\n'), DEFAULT_AFTER_LOGIN);
});

test('nilai bukan string atau kosong memakai fallback', () => {
  assert.equal(sanitizeNext(undefined), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext(null), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext(['/queues']), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext(''), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext(' /dashboard'), DEFAULT_AFTER_LOGIN);
});

test('beranda dan parameter login tidak dipakai sebagai tujuan (hindari loop login)', () => {
  assert.equal(sanitizeNext('/'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('/?next=%2Fqueues'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('/login'), DEFAULT_AFTER_LOGIN);
  assert.equal(sanitizeNext('/login?next=%2Fqueues'), DEFAULT_AFTER_LOGIN);
});

test('fallback yang diberikan pemanggil dipakai, termasuk string kosong', () => {
  assert.equal(sanitizeNext('https://evil.example', '/ticket/5'), '/ticket/5');
  assert.equal(sanitizeNext(undefined, ''), '');
  assert.equal(sanitizeNext('/queues', ''), '/queues');
});
