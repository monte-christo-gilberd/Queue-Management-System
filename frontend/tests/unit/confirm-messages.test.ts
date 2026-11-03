/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { confirmCopy } from '@/utils/confirm-messages';

test('Next menyebut nomor yang akan selesai dilayani', () => {
  const c = confirmCopy('next', 'A-003');
  assert.equal(c.title, 'Lanjut ke nomor berikutnya?');
  assert.match(c.message, /A-003 akan ditandai selesai dilayani/);
  assert.equal(c.confirmLabel, 'Ya, Lanjut');
});

test('Next pada counter kosong memanggil nomor pertama', () => {
  const c = confirmCopy('next', null);
  assert.equal(c.title, 'Panggil nomor pertama?');
  assert.doesNotMatch(c.message, /A-/);
});

test('Skip menyebut nomor yang dilewati', () => {
  const c = confirmCopy('skip', 'A-004');
  assert.equal(c.title, 'Lewati A-004?');
  assert.match(c.message, /A-004 akan dilewati/);
  assert.equal(c.confirmLabel, 'Ya, Lewati');
});

test('Skip pada counter kosong menjelaskan tidak ada yang dilayani', () => {
  assert.match(confirmCopy('skip', null).message, /Belum ada nomor yang sedang dilayani/);
});

test('nama counter muncul hanya bila diberikan', () => {
  assert.match(confirmCopy('next', 'A-001', 'Counter 2').message, /di Counter 2/);
  assert.match(confirmCopy('skip', 'A-001', 'Counter 2').message, /di Counter 2/);
  assert.doesNotMatch(confirmCopy('next', 'A-001').message, / di /);
  assert.doesNotMatch(confirmCopy('next', 'A-001', null).message, / di /);
});
