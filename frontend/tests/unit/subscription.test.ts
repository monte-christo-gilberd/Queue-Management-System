/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describeSubscriptionError } from '@/lib/subscription';

test('layanan langganan tidak tersedia (nginx 502/503/504 atau jaringan): pesan ramah, antrian tetap berjalan', () => {
  for (const e of [
    new ApiError(503, 'UNKNOWN', 'x'),
    new ApiError(502, 'UNKNOWN', 'x'),
    new ApiError(504, 'UNKNOWN', 'x'),
    new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server'),
  ]) {
    assert.match(describeSubscriptionError(e), /sementara tidak tersedia.*Antrian Anda tetap berjalan/);
  }
});

test('galat lain dipetakan terpisah', () => {
  assert.match(describeSubscriptionError(new ApiError(429, 'UNKNOWN', 'x')), /Terlalu banyak/);
  assert.match(describeSubscriptionError(new ApiError(403, 'UNKNOWN_USER', 'x')), /login ulang/);
  assert.match(describeSubscriptionError(new ApiError(500, 'INTERNAL_ERROR', 'x')), /kesalahan di server/);
  assert.match(describeSubscriptionError(new Error('boom')), /Terjadi kesalahan/);
});
