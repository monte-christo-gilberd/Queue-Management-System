/// <reference types="node" />
// Sumber: product-overview.md — 4.2 edge case (antrian ditutup, sudah bergabung, offline, antrian penuh),
// 4.3 edge case (Next pada antrian kosong), 4.18 / US-21 (langganan tidak aktif → diblokir dan diarahkan bayar).
// Kode error mengikuti backend (internal/handler, internal/repository).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describeInsertError, describeQueueError } from '@/lib/queue-errors';

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

test('token QR tidak valid / sudah diganti: minta pindai QR terbaru (4.2 penggantian)', () => {
  assert.equal(
    describeQueueError(apiError('QR_INVALID')),
    'This QR link is invalid or no longer valid. Scan the latest QR from the queue owner.',
  );
});

test('token QR dinamis kedaluwarsa: minta pindai ulang (4.2 dinamis)', () => {
  assert.equal(describeQueueError(apiError('QR_EXPIRED')), 'This QR link has expired. Scan the latest QR again.');
});

test('qr-service tidak tersedia: pesan layanan QR sedang down (4.2)', () => {
  assert.equal(describeQueueError(apiError('QR_UNAVAILABLE')), 'The QR service is unavailable right now. Please try again.');
});

test('mode QR tidak valid dan penggantian dinamis ditolak dengan pesan jelas (4.2)', () => {
  assert.equal(describeQueueError(apiError('QR_TYPE_INVALID')), 'Unknown QR mode. Choose static or dynamic.');
  assert.equal(
    describeQueueError(apiError('QR_REPLACE_NOT_ALLOWED')),
    'A dynamic QR cannot be replaced. Let it expire; it refreshes automatically.',
  );
});

test('gagal terhubung ke server menampilkan pesan koneksi dari ApiError (4.2 offline)', () => {
  const error = new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server');
  assert.equal(describeQueueError(error), 'Tidak dapat terhubung ke server');
});

test('kode lain dari server memakai pesan kesalahan server umum', () => {
  assert.equal(describeQueueError(apiError('INTERNAL_ERROR')), 'Terjadi kesalahan di server, coba lagi');
  assert.equal(describeQueueError(apiError('SOMETHING_NEW')), 'Terjadi kesalahan di server, coba lagi');
});

test('kode QR tak dikenal dan error non-API jatuh ke pesan QR umum', async () => {
  const { describeQrError, QR_DEFAULT_MESSAGE } = await import('@/lib/qr-errors');
  assert.equal(describeQrError(apiError('SOMETHING_ELSE')), QR_DEFAULT_MESSAGE);
  assert.equal(describeQrError(new Error('boom')), QR_DEFAULT_MESSAGE);
});

test('sisip: username ganda menyarankan email (US-10)', () => {
  assert.equal(describeInsertError(apiError('AMBIGUOUS_ACCOUNT')), 'Lebih dari satu akun memakai username itu. Gunakan emailnya.');
});

test('sisip: akun tidak ditemukan dan akun sudah ikut antrian dijelaskan ke owner', () => {
  assert.equal(describeInsertError(apiError('NOT_FOUND')), 'Akun tidak ditemukan. Periksa email atau username.');
  assert.equal(describeInsertError(apiError('ALREADY_JOINED')), 'Akun ini sudah ikut antrian.');
});

test('sisip: kode lain jatuh ke pesan antrian umum', () => {
  assert.equal(describeInsertError(apiError('QUEUE_CLOSED')), describeQueueError(apiError('QUEUE_CLOSED')));
  assert.equal(describeInsertError(new Error('x')), 'Terjadi kesalahan, coba lagi');
});
