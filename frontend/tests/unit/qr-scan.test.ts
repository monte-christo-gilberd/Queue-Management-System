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
