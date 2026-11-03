/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import type { Counter } from '@/lib/counters';
import { resolveDeviceCounter, switchNotice } from '@/utils/device-counter';

const counter = (id: number, name: string, isDefault = false, serving: Counter['serving'] = null): Counter => ({
  id,
  name,
  is_default: isDefault,
  serving,
});

const counters = [counter(1, 'Counter 1', true), counter(2, 'Counter 2'), counter(3, 'Loket B')];

test('tanpa pilihan tersimpan, perangkat memakai counter default', () => {
  assert.equal(resolveDeviceCounter(null, counters), 1);
});

test('pilihan tersimpan dipakai bila counter itu masih ada', () => {
  assert.equal(resolveDeviceCounter(3, counters), 3);
});

test('pilihan yang sudah dihapus dari perangkat lain jatuh ke default', () => {
  assert.equal(resolveDeviceCounter(9, counters), 1);
});

test('daftar belum dimuat: 0 (server memakai default)', () => {
  assert.equal(resolveDeviceCounter(2, []), 0);
});

test('tanpa counter default, jatuh ke counter pertama', () => {
  assert.equal(resolveDeviceCounter(null, [counter(5, 'Counter 2'), counter(6, 'Loket B')]), 5);
});

test('pindah dari counter kosong tidak memberi peringatan', () => {
  assert.equal(switchNotice(counter(2, 'Counter 2')), null);
  assert.equal(switchNotice(undefined), null);
});

test('pindah dari counter yang masih melayani memberi peringatan dengan nomornya', () => {
  const notice = switchNotice(counter(2, 'Counter 2', false, { ticket_id: 7, ticket_number: 4 }));
  assert.equal(notice, 'Counter 2 masih melayani 4; pilih Counter 2 lagi untuk melanjutkan.');
});
