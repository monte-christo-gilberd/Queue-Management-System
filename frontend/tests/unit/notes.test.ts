/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { notePreview } from '@/utils/notes';

test('catatan pendek ditampilkan apa adanya', () => {
  assert.equal(notePreview('minta struk'), 'minta struk');
});

test('spasi dan baris baru dipadatkan menjadi satu baris', () => {
  assert.equal(notePreview('  minta   struk\n\ndan  faktur '), 'minta struk dan faktur');
});

test('catatan panjang dipotong dengan elipsis pada batas', () => {
  const long = 'a'.repeat(200);
  const out = notePreview(long, 20);
  assert.equal(out.length, 20);
  assert.ok(out.endsWith('…'));
  assert.equal(notePreview('a'.repeat(20), 20), 'a'.repeat(20), 'tepat pada batas: tidak dipotong');
});
