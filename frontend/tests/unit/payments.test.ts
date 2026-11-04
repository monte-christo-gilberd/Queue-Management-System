/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describePaymentError } from '@/lib/payments';

test('payment-service mati: pesan ramah, antrian dinyatakan tetap berjalan', () => {
  for (const e of [
    new ApiError(503, 'PAYMENT_UNAVAILABLE', 'x'),
    new ApiError(502, 'PAYMENT_PROVIDER_ERROR', 'x'),
    new ApiError(504, 'UNKNOWN', 'x'), // dari nginx tanpa body JSON
    new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server'),
  ]) {
    assert.match(describePaymentError(e), /sementara tidak tersedia.*Antrian Anda tetap berjalan/);
  }
});

test('galat lain dipetakan terpisah', () => {
  assert.match(describePaymentError(new ApiError(429, 'UNKNOWN', 'x')), /Terlalu banyak/);
  assert.match(describePaymentError(new ApiError(403, 'NOT_OWNER', 'x')), /Hanya owner/);
  assert.match(describePaymentError(new ApiError(500, 'INTERNAL', 'x')), /kesalahan di server/);
  assert.match(describePaymentError(new Error('boom')), /Terjadi kesalahan/);
});
