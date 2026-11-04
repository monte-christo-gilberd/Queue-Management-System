/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError } from '@/lib/api';
import { describeDisplayError, type DisplaySnapshot } from '@/lib/display';
import {
  buildCards,
  changedCounterIds,
  FETCH_FRESH_MS,
  formatClock,
  isConnected,
  layoutColumns,
  numberFontSize,
  SSE_FRESH_MS,
} from '@/utils/display';

function snap(counters: { id: number; name: string; serving: string | null }[], multi = counters.length > 1): DisplaySnapshot {
  return {
    queue_name: 'Loket A',
    queue_desc: null,
    status: 'active',
    multi_counter: multi,
    counters,
    upcoming: [],
    skipped: [],
    generated_at: '2026-11-04T10:00:00Z',
  };
}

test('satu kartu per counter, urutan dari server', () => {
  const cards = buildCards(snap([{ id: 5, name: 'Counter 1', serving: 'A-001' }, { id: 9, name: 'Cepat', serving: null }]));
  assert.deepEqual(cards.map((c) => c.id), [5, 9]);
  assert.deepEqual(cards.map((c) => c.number), ['A-001', null]);
});

test('nama counter hanya tampil bila antrian punya lebih dari satu counter', () => {
  const single = buildCards(snap([{ id: 1, name: 'Counter 1', serving: 'A-001' }]));
  assert.equal(single[0].title, null);
  const multi = buildCards(snap([{ id: 1, name: 'Counter 1', serving: null }, { id: 2, name: 'Loket Cepat', serving: null }]));
  assert.deepEqual(multi.map((c) => c.title), ['Counter 1', 'Loket Cepat']);
});

test('counter ditambah atau dihapus langsung mengubah jumlah kartu', () => {
  const before = snap([{ id: 1, name: 'a', serving: null }]);
  const after = snap([{ id: 1, name: 'a', serving: null }, { id: 2, name: 'b', serving: null }, { id: 3, name: 'c', serving: null }]);
  assert.equal(buildCards(before).length, 1);
  assert.equal(buildCards(after).length, 3);
  assert.equal(buildCards(snap([{ id: 1, name: 'a', serving: null }, { id: 3, name: 'c', serving: null }])).length, 2);
});

test('sorotan: hanya counter yang nomornya berganti, snapshot pertama tidak menyorot', () => {
  const first = snap([{ id: 1, name: 'a', serving: 'A-001' }, { id: 2, name: 'b', serving: 'A-002' }]);
  assert.deepEqual(changedCounterIds(null, first), []);
  const next = snap([{ id: 1, name: 'a', serving: 'A-003' }, { id: 2, name: 'b', serving: 'A-002' }]);
  assert.deepEqual(changedCounterIds(first, next), [1]);
});

test('sorotan: counter yang berhenti melayani atau tidak berubah tidak disorot, counter baru yang melayani disorot', () => {
  const prev = snap([{ id: 1, name: 'a', serving: 'A-001' }]);
  assert.deepEqual(changedCounterIds(prev, snap([{ id: 1, name: 'a', serving: null }])), []);
  assert.deepEqual(changedCounterIds(prev, prev), []);
  const withNew = snap([{ id: 1, name: 'a', serving: 'A-001' }, { id: 2, name: 'b', serving: 'A-009' }]);
  assert.deepEqual(changedCounterIds(prev, withNew), [2]);
});

test('kolom: satu kartu satu kolom, layar tegak paling banyak dua kolom, mendatar menyebar', () => {
  assert.equal(layoutColumns(1, true), 1);
  assert.equal(layoutColumns(2, true), 2);
  assert.equal(layoutColumns(4, true), 2);
  assert.equal(layoutColumns(10, true), 5);
  for (let n = 1; n <= 10; n++) assert.ok(layoutColumns(n, false) <= 2, `tegak ${n}`);
  assert.equal(layoutColumns(0, true), 1);
  assert.equal(layoutColumns(99, true), 5, 'dibatasi 10 kartu');
});

test('ukuran huruf nomor menyusut seiring jumlah kartu dan tetap dalam batas', () => {
  const area = { areaWidth: 1872, areaHeight: 700, landscape: true };
  const one = numberFontSize({ count: 1, ...area });
  const ten = numberFontSize({ count: 10, ...area });
  assert.ok(one > ten, `${one} > ${ten}`);
  for (const n of [1, 2, 3, 5, 10]) {
    const f = numberFontSize({ count: n, ...area });
    assert.ok(f >= 28 && f <= 240, `count ${n}: ${f}`);
  }
  assert.equal(numberFontSize({ count: 1, areaWidth: 100, areaHeight: 100, landscape: true }), 28, 'layar sangat kecil: batas bawah');
});

test('10 kartu muat di layar TV 1920x1080: huruf nomor tidak melebihi lebar kartu', () => {
  const areaWidth = 1920 - 48;
  const areaHeight = 1080 - 48 - 96 - 240;
  const columns = layoutColumns(10, true);
  const cardWidth = (areaWidth - 16 * (columns - 1)) / columns;
  const f = numberFontSize({ count: 10, areaWidth, areaHeight, landscape: true });
  assert.ok(f * 0.62 * 5 <= cardWidth, `lebar teks ${f * 0.62 * 5} <= kartu ${cardWidth}`);
});

test('tersambung bila stream berdetak atau snapshot baru berhasil diambil', () => {
  const now = 1_000_000;
  assert.equal(isConnected(now, now - 1000, null), true);
  assert.equal(isConnected(now, null, now - 1000), true);
  assert.equal(isConnected(now, now - SSE_FRESH_MS - 1, now - FETCH_FRESH_MS - 1), false, 'keduanya basi');
  assert.equal(isConnected(now, null, null), false);
  assert.equal(isConnected(now, now - SSE_FRESH_MS - 1, now - 500), true, 'stream basi tetapi polling berjalan');
});

test('jam berformat 24 jam dua digit', () => {
  assert.equal(formatClock(new Date(2026, 10, 4, 7, 5)), '07:05');
  assert.equal(formatClock(new Date(2026, 10, 4, 17, 12)), '17:12');
});

test('pesan galat pengaturan layar', () => {
  assert.match(describeDisplayError(new ApiError(400, 'BAD_REQUEST', 'x')), /antara 0 dan 10/);
  assert.match(describeDisplayError(new ApiError(404, 'NOT_FOUND', 'x')), /tidak ditemukan/);
  assert.match(describeDisplayError(new ApiError(0, 'NETWORK_ERROR', 'Tidak bisa terhubung')), /terhubung/);
  assert.match(describeDisplayError(new Error('x')), /Terjadi kesalahan/);
});
