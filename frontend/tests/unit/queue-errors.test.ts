/// <reference types="node" />
// Sumber: product-overview.md — 4.2 edge case (antrian ditutup, sudah bergabung, offline, antrian penuh),
// 4.3 edge case (Next pada antrian kosong), 4.18 / US-21 (langganan tidak aktif → diblokir dan diarahkan bayar).
// Kode error mengikuti backend (internal/handler, internal/repository).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describeQueueError } from '@/lib/queue-errors';

const apiError = (code: string, message = 'server message') => new ApiError(400, code, message);

test('error bukan dari API ditampilkan sebagai pesan umum', () => {
  assert.equal(describeQueueError(new Error('boom')), 'Terjadi kesalahan, coba lagi');
  assert.equal(describeQueueError('string'), 'Terjadi kesalahan, coba lagi');
  assert.equal(describeQueueError(undefined), 'Terjadi kesalahan, coba lagi');
});

test('langganan tidak aktif: pesan mengarahkan owner untuk mengaktifkan langganan (US-21 AC1, 4.18)', () => {
  const message = describeQueueError(apiError('SUBSCRIPTION_INACTIVE'));
  assert.equal(message, 'Langganan Anda tidak aktif. Aktifkan langganan untuk membuat atau mengubah antrian.');
});

test('antrian tidak ditemukan: pesan tidak membocorkan detail (TC-05-06, TC-08-03)', () => {
  assert.equal(describeQueueError(apiError('NOT_FOUND')), 'Antrian tidak ditemukan');
});

test('Next pada antrian kosong: pesan menyatakan tidak ada yang menunggu (4.3 edge case)', () => {
  assert.equal(describeQueueError(apiError('QUEUE_EMPTY')), 'Tidak ada yang menunggu di antrian ini');
});

test('antrian sudah ditutup: pesan menyatakan antrian ditutup (4.2 edge case)', () => {
  assert.equal(describeQueueError(apiError('QUEUE_CLOSED')), 'Antrian sudah ditutup');
});

test('sudah terdaftar di antrian: pesan menyebut sudah terdaftar (4.2 edge case, tidak membuat nomor baru)', () => {
  assert.equal(describeQueueError(apiError('ALREADY_JOINED')), 'Anda sudah terdaftar di antrian ini');
});

test('server sibuk (retry habis) meminta pengguna mencoba lagi (NFR 3)', () => {
  assert.equal(describeQueueError(apiError('RETRY_EXHAUSTED')), 'Server sedang sibuk, silakan coba lagi');
});

test('input tidak valid meminta pengguna memeriksa isian (TC-08-06, TC-02-02)', () => {
  assert.equal(describeQueueError(apiError('BAD_REQUEST')), 'Data tidak valid, periksa kembali isian Anda');
});

test('gagal terhubung ke server menampilkan pesan koneksi dari ApiError (4.2 offline)', () => {
  const error = new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server');
  assert.equal(describeQueueError(error), 'Tidak dapat terhubung ke server');
});

test('kode lain dari server memakai pesan kesalahan server umum', () => {
  assert.equal(describeQueueError(apiError('INTERNAL_ERROR')), 'Terjadi kesalahan di server, coba lagi');
  assert.equal(describeQueueError(apiError('SOMETHING_NEW')), 'Terjadi kesalahan di server, coba lagi');
});
