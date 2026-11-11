/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { CONFIRM_PASSWORD_MISMATCH, getConfirmPasswordError } from '@/utils/confirm-password';

test('selama pengguna belum selesai mengisi, tidak ada pesan walau isinya beda', () => {
  // Karakter pertama di field konfirmasi selalu berbeda dari password yang lebih panjang.
  assert.equal(getConfirmPasswordError('abcdefgh', 'a', false), undefined);
  assert.equal(getConfirmPasswordError('abcdefgh', 'abcdefgx', false), undefined);
});

test('setelah blur atau submit, isi yang berbeda menampilkan pesan', () => {
  assert.equal(getConfirmPasswordError('abcdefgh', 'abc', true), CONFIRM_PASSWORD_MISMATCH);
  assert.equal(getConfirmPasswordError('abcdefgh', 'abcdefgx', true), CONFIRM_PASSWORD_MISMATCH);
});

test('pesan hilang begitu isi konfirmasi sama dengan password', () => {
  assert.equal(getConfirmPasswordError('abcdefgh', 'abcdefgh', true), undefined);
});

test('field konfirmasi kosong tidak memunculkan pesan (ditangani validasi kirim)', () => {
  assert.equal(getConfirmPasswordError('abcdefgh', '', true), undefined);
  assert.equal(getConfirmPasswordError('', '', true), undefined);
});

test('pesan yang tampil adalah teks yang sama dengan sebelumnya', () => {
  assert.equal(CONFIRM_PASSWORD_MISMATCH, 'Password tidak sama');
});
