/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describeProfileError, subscriptionStatus, validateUsername } from '@/utils/profile';

test('username kosong atau hanya spasi ditolak', () => {
  assert.ok(validateUsername(''));
  assert.ok(validateUsername('   '));
});

test('username dibatasi 50 karakter per karakter, bukan per byte', () => {
  assert.equal(validateUsername('a'.repeat(50)), null);
  assert.ok(validateUsername('a'.repeat(51)));
  assert.equal(validateUsername('é'.repeat(50)), null);
});

test('username dengan tanda < atau > ditolak (sama dengan backend)', () => {
  assert.ok(validateUsername('<b>x'));
  assert.equal(validateUsername('Budi Santoso'), null);
});

test('langganan: belum pernah, aktif, dan berakhir', () => {
  const now = new Date('2026-11-04T00:00:00Z');
  assert.equal(subscriptionStatus(null, now).active, false);
  const active = subscriptionStatus('2026-12-01T00:00:00Z', now);
  assert.equal(active.active, true);
  assert.match(active.description, /^Aktif hingga /);
  const ended = subscriptionStatus('2026-10-01T00:00:00Z', now);
  assert.equal(ended.active, false);
  assert.match(ended.description, /^Langganan berakhir /);
});

test('pesan galat profil', () => {
  assert.match(describeProfileError(new ApiError(400, 'BAD_REQUEST', 'x')), /tidak valid/);
  assert.equal(describeProfileError(new ApiError(0, 'NETWORK_ERROR', 'Tidak bisa terhubung')), 'Tidak bisa terhubung');
  assert.match(describeProfileError(new ApiError(500, 'INTERNAL_ERROR', 'x')), /server/);
  assert.match(describeProfileError(new Error('x')), /Terjadi kesalahan/);
});
