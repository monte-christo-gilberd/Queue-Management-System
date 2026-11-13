/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { parseJoinToken } from '@/lib/qr-scan';

test('URL join valid menghasilkan token', () => {
  assert.equal(parseJoinToken('http://localhost:8081/join?t=v1.abc.def'), 'v1.abc.def');
  assert.equal(parseJoinToken(' https://app.example.com/join/?t=xyz '), 'xyz');
});

test('QR yang bukan QR antrian ditolak', () => {
  assert.equal(parseJoinToken('hello'), null);
  assert.equal(parseJoinToken('javascript:alert(1)'), null);
  assert.equal(parseJoinToken('http://x.com/other?t=abc'), null);
  assert.equal(parseJoinToken('http://x.com/join'), null);
  assert.equal(parseJoinToken('http://x.com/join?t='), null);
  assert.equal(parseJoinToken(`http://x.com/join?t=${'a'.repeat(3000)}`), null);
});

// Kode 6 karakter yang diketik manual diterima, termasuk huruf kecil dan spasi di sekitarnya.
test('kode 6 karakter yang diketik diterima dan dikapitalkan', () => {
  assert.equal(parseJoinToken('ab23cd'), 'AB23CD');
  assert.equal(parseJoinToken('  AB23CD  '), 'AB23CD');
});

// Kode dengan karakter yang dikecualikan atau panjang yang salah ditolak.
test('kode yang salah format ditolak', () => {
  assert.equal(parseJoinToken('AB0CD2'), null, 'angka 0 tidak dipakai');
  assert.equal(parseJoinToken('AB1CD2'), null, 'angka 1 tidak dipakai');
  assert.equal(parseJoinToken('ABOCD2'), null, 'huruf O tidak dipakai');
  assert.equal(parseJoinToken('AB23C'), null, 'terlalu pendek');
  assert.equal(parseJoinToken('AB23CDE'), null, 'terlalu panjang');
});
