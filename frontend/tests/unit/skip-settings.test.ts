/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { validateSkipSettings } from '@/lib/skip-settings';

test('remove tidak butuh N dan mengirim n null', () => {
  const r = validateSkipSettings({ fallback: 'remove', n: '', maxReturns: '1' });
  assert.deepEqual(r, { ok: true, value: { fallback: 'remove', n: null, max_returns: 1 } });
});

test('after_n membawa N', () => {
  const r = validateSkipSettings({ fallback: 'after_n', n: '3', maxReturns: '2' });
  assert.deepEqual(r, { ok: true, value: { fallback: 'after_n', n: 3, max_returns: 2 } });
});

test('N kosong, 0, negatif, desimal atau huruf ditolak untuk after_n', () => {
  for (const n of ['', '0', '-1', '1.5', 'a']) {
    assert.equal(validateSkipSettings({ fallback: 'after_n', n, maxReturns: '1' }).ok, false, n);
  }
});

test('N diabaikan untuk mode lain', () => {
  assert.equal(validateSkipSettings({ fallback: 'back', n: 'abc', maxReturns: '1' }).ok, true);
});

test('batas pengembalian harus 1 atau lebih', () => {
  for (const m of ['', '0', '-2', 'x']) {
    assert.equal(validateSkipSettings({ fallback: 'back', n: '', maxReturns: m }).ok, false, m);
  }
});
