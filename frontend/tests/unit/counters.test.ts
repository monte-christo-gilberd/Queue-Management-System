/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { MAX_COUNTER_NAME_LENGTH, validateCounterName } from '@/lib/counters';

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
