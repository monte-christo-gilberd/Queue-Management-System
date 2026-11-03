/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { MAX_COUNTER_NAME_LENGTH, suggestCounterName, validateCounterName } from '@/lib/counters';

test('nama kosong ditolak kecuali nama boleh kosong (buat otomatis)', () => {
  assert.equal(validateCounterName('   ', false), 'Nama counter wajib diisi');
  assert.equal(validateCounterName('', true), null);
});

test('nama tepat batas diterima, lebih dari batas ditolak', () => {
  assert.equal(validateCounterName('a'.repeat(MAX_COUNTER_NAME_LENGTH), false), null);
  assert.match(validateCounterName('a'.repeat(MAX_COUNTER_NAME_LENGTH + 1), false) ?? '', /maksimal 30/);
});

test('panjang dihitung per karakter (bukan per byte)', () => {
  assert.equal(validateCounterName('ö'.repeat(MAX_COUNTER_NAME_LENGTH), false), null);
});

test('saran nama: Counter 2 bila baru ada default', () => {
  assert.equal(suggestCounterName(['Counter 1']), 'Counter 2');
});

test('saran nama: melompati nomor yang sudah dipakai', () => {
  assert.equal(suggestCounterName(['Counter 1', 'Counter 2']), 'Counter 3');
  assert.equal(suggestCounterName(['Counter 1', 'Counter 3']), 'Counter 2');
});

test('saran nama: nama yang diganti pemilik tidak dihitung sebagai pemakaian', () => {
  assert.equal(suggestCounterName(['Loket B']), 'Counter 2');
});
