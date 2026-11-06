/// <reference types="node" />
// Verifikasi email: pesan galat dan pengenal token tidak sah. Kode galat mengikuti backend
// (internal/handler/email_verification_handler.go).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import {
  describeResendError,
  describeVerifyError,
  isInvalidVerifyToken,
  resendSuccessMessage,
} from '@/lib/email-verification';
import { describeQueueError } from '@/lib/queue-errors';

test('tautan verifikasi tidak sah dikenali dari kode INVALID_TOKEN saja', () => {
  assert.equal(isInvalidVerifyToken(new ApiError(400, 'INVALID_TOKEN', 'x')), true);
  assert.equal(isInvalidVerifyToken(new ApiError(400, 'BAD_REQUEST', 'x')), false);
  assert.equal(isInvalidVerifyToken(new ApiError(500, 'INTERNAL_ERROR', 'x')), false);
  assert.equal(isInvalidVerifyToken(new Error('boom')), false);
});

test('kirim ulang: batas per jam dari backend (429) dijelaskan, bukan pesan umum', () => {
  assert.match(describeResendError(new ApiError(429, 'TOO_MANY_REQUESTS', 'x')), /satu jam/);
});

test('kirim ulang: 429 dari nginx tanpa body JSON juga dikenali lewat status', () => {
  assert.match(describeResendError(new ApiError(429, 'UNKNOWN', '')), /satu jam/);
});

test('kirim ulang: galat jaringan memakai pesan dari klien API', () => {
  assert.equal(describeResendError(new ApiError(0, 'NETWORK_ERROR', 'Tidak ada koneksi')), 'Tidak ada koneksi');
});

test('kirim ulang: galat server dan galat bukan API punya pesan sendiri', () => {
  assert.match(describeResendError(new ApiError(500, 'INTERNAL_ERROR', 'x')), /tidak bisa dikirim/);
  assert.equal(describeResendError(new Error('boom')), 'Terjadi kesalahan, coba lagi');
});

test('halaman verifikasi: pesan untuk jaringan, rate limit, dan server', () => {
  assert.equal(describeVerifyError(new ApiError(0, 'NETWORK_ERROR', 'Tidak ada koneksi')), 'Tidak ada koneksi');
  assert.match(describeVerifyError(new ApiError(429, 'UNKNOWN', '')), /Terlalu banyak/);
  assert.match(describeVerifyError(new ApiError(500, 'INTERNAL_ERROR', 'x')), /server/);
  assert.equal(describeVerifyError('string'), 'Terjadi kesalahan, coba lagi');
});

test('pesan sukses menyebut alamat email tujuan', () => {
  assert.equal(resendSuccessMessage('a@b.com'), 'Tautan verifikasi sudah dikirim ke a@b.com');
});

test('EMAIL_NOT_VERIFIED di alur gabung dan buat antrian mengarahkan ke halaman Profil', () => {
  const message = describeQueueError(new ApiError(403, 'EMAIL_NOT_VERIFIED', 'verify your email'));
  assert.match(message, /Verifikasi email/);
  assert.match(message, /Profil/);
});
