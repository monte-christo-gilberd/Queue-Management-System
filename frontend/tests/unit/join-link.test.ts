/// <reference types="node" />
// Sumber: product-overview.md — 4.2 langkah 1 (QR statis menuju halaman join antrian), 4.2 edge case
// (tautan langsung sama dengan hasil scan), TC-05-08 / TC-05-09 (tempel tautan manual).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { buildJoinLink, parseQueueIdFromLink } from '@/lib/join-link';

test('tautan join dibangun dari origin web dan id antrian', () => {
  assert.equal(buildJoinLink(12, 'http://192.168.1.5:8081'), 'http://192.168.1.5:8081/join/12');
});

test('garis miring di akhir origin tidak menghasilkan //join', () => {
  assert.equal(buildJoinLink(12, 'https://qms.example///'), 'https://qms.example/join/12');
});

test('tanpa origin (native tanpa EXPO_PUBLIC_WEB_URL) tautan tidak tersedia', () => {
  assert.equal(buildJoinLink(12, null), null);
  assert.equal(buildJoinLink(12, ''), null);
});

test('id antrian yang bukan bilangan bulat positif tidak menghasilkan tautan', () => {
  for (const id of [0, -1, 1.5, Number.NaN]) {
    assert.equal(buildJoinLink(id, 'https://qms.example'), null, `id=${id}`);
  }
});

test('tautan lengkap, path, dan angka saja dikenali sebagai id antrian yang sama', () => {
  assert.equal(parseQueueIdFromLink('https://qms.example/join/12'), 12);
  assert.equal(parseQueueIdFromLink('/join/12'), 12);
  assert.equal(parseQueueIdFromLink('12'), 12);
});

test('query string, fragment, dan garis miring akhir diabaikan', () => {
  assert.equal(parseQueueIdFromLink('/join/12/'), 12);
  assert.equal(parseQueueIdFromLink('https://qms.example/join/12?src=qr'), 12);
  assert.equal(parseQueueIdFromLink('https://qms.example/join/12#top'), 12);
});

test('spasi di sekitar tautan manual diabaikan', () => {
  assert.equal(parseQueueIdFromLink('  https://qms.example/join/7  '), 7);
});

test('teks yang bukan tautan join ditolak (TC-05-09 "bukan-tautan")', () => {
  assert.equal(parseQueueIdFromLink('bukan-tautan'), null);
  assert.equal(parseQueueIdFromLink(''), null);
  assert.equal(parseQueueIdFromLink('   '), null);
});

test('id tidak valid dalam tautan ditolak (abc, 0, angka tidak aman)', () => {
  assert.equal(parseQueueIdFromLink('/join/abc'), null);
  assert.equal(parseQueueIdFromLink('/join/0'), null);
  assert.equal(parseQueueIdFromLink('/join/12abc'), null);
  assert.equal(parseQueueIdFromLink('/join/99999999999999999999999'), null);
  assert.equal(parseQueueIdFromLink('-5'), null);
});

test('tautan yang dibangun lalu di-parse kembali menghasilkan id yang sama', () => {
  const link = buildJoinLink(345, 'http://10.0.0.2:8081');
  assert.notEqual(link, null);
  assert.equal(parseQueueIdFromLink(link as string), 345);
});
