/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describeForgotError, describeResetError, isInvalidResetToken } from '@/lib/password-reset';
import { validatePassword } from '@/utils/password';

test('password kurang dari 8 karakter ditolak, 8 diterima', () => {
  assert.ok(validatePassword('1234567'));
  assert.equal(validatePassword('12345678'), null);
  assert.ok(validatePassword(''));
});

test('panjang dihitung per karakter, batas atas per byte (bcrypt 72)', () => {
  assert.equal(validatePassword('pässwörd'), null);
  assert.equal(validatePassword('a'.repeat(72)), null);
  assert.ok(validatePassword('a'.repeat(73)));
  assert.ok(validatePassword('é'.repeat(37)), '37 karakter = 74 byte');
});

test('galat permintaan tautan: 429 dari nginx dikenali lewat status', () => {
  assert.match(describeForgotError(new ApiError(429, 'UNKNOWN', 'x')), /Terlalu banyak/);
  assert.match(describeForgotError(new ApiError(400, 'BAD_REQUEST', 'x')), /Email tidak valid/);
  assert.match(describeForgotError(new ApiError(0, 'NETWORK_ERROR', 'Tidak bisa terhubung')), /terhubung/);
  assert.match(describeForgotError(new Error('x')), /Terjadi kesalahan/);
});

test('galat simpan password: lemah dan token tidak sah dibedakan', () => {
  assert.match(describeResetError(new ApiError(400, 'WEAK_PASSWORD', 'x')), /minimal 8/);
  assert.equal(isInvalidResetToken(new ApiError(400, 'INVALID_TOKEN', 'x')), true);
  assert.equal(isInvalidResetToken(new ApiError(400, 'WEAK_PASSWORD', 'x')), false);
  assert.equal(isInvalidResetToken(new Error('x')), false);
});
