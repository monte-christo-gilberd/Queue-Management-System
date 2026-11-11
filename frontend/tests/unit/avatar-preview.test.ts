/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { getAvatarPreviewSize } from '@/utils/avatar-preview';

test('layar lebar: foto dibatasi lebar bagian dalam kartu popup compact (464)', () => {
  // 480 (lebar maksimum kartu) - 16 (padding compact kiri dan kanan)
  assert.equal(getAvatarPreviewSize(1920, 1080), 464);
  assert.equal(getAvatarPreviewSize(1280, 900), 464);
});

test('layar sempit: foto mengikuti lebar layar dikurangi jarak latar dan padding kartu', () => {
  // 390 - 60 (latar) - 16 (padding kartu) = 314
  assert.equal(getAvatarPreviewSize(390, 844), 314);
  assert.equal(getAvatarPreviewSize(360, 800), 284);
});

test('layar pendek (landscape): foto mengikuti tinggi dikurangi padding kartu dan ruang aman', () => {
  // 400 - 64 = 336, lebih kecil daripada batas lebar (464)
  assert.equal(getAvatarPreviewSize(800, 400), 336);
});

test('hasil tidak pernah negatif dan selalu bilangan bulat', () => {
  assert.equal(getAvatarPreviewSize(50, 50), 0);
  assert.equal(getAvatarPreviewSize(0, 0), 0);
  assert.ok(Number.isInteger(getAvatarPreviewSize(391.7, 843.3)));
});
