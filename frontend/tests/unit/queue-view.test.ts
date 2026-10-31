/// <reference types="node" />
// Sumber: product-overview.md — US-8 dan 4.3 (Next memajukan nomor yang dilayani; tidak ada nomor berikutnya
// berarti antrian kosong), 4.7 / 4.14 (skipped tidak lagi menunggu), 4.9 (dibatalkan keluar dari urutan),
// 4.12 (sisipan di posisi 1 menjadi "berikutnya", tidak menginterupsi yang sedang dilayani).
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { deriveQueueView } from '@/utils/queue-view';
import type { Ticket } from '@/types/api';

function ticket(id: number, overrides: Partial<Ticket> = {}): Ticket {
  return {
    id,
    queue_id: 10,
    ticket_number: id,
    sort_key: id,
    user_id: 100 + id,
    status: 'waiting',
    skip_count: 0,
    last_notified_at: null,
    created_at: '2026-10-17T08:00:00Z',
    completed_at: null,
    cancelled_at: null,
    ...overrides,
  };
}

function served(id: number, completedAt: string, overrides: Partial<Ticket> = {}): Ticket {
  return ticket(id, { status: 'completed', completed_at: completedAt, ...overrides });
}

test('antrian kosong: tidak ada yang dilayani, tidak ada berikutnya (4.3 langkah 4)', () => {
  assert.deepEqual(deriveQueueView([]), { previous: null, current: null, next: null, upcoming: [] });
});

test('antrian belum dimulai: nomor pertama menjadi berikutnya, sisanya upcoming sesuai urutan bergabung (US-14)', () => {
  const view = deriveQueueView([ticket(3), ticket(1), ticket(2)]);
  assert.equal(view.current, null);
  assert.equal(view.previous, null);
  assert.equal(view.next?.ticket.id, 1);
  assert.deepEqual(view.upcoming.map((e) => e.ticket.id), [2, 3]);
});

test('setelah NEXT pertama: nomor pertama menjadi sedang dilayani (US-8)', () => {
  const view = deriveQueueView([served(1, '2026-10-17T09:00:00Z'), ticket(2), ticket(3)]);
  assert.equal(view.current?.ticket.id, 1);
  assert.equal(view.previous, null);
  assert.equal(view.next?.ticket.id, 2);
  assert.deepEqual(view.upcoming.map((e) => e.ticket.id), [3]);
});

test('setelah NEXT kedua: yang sedang dilayani maju dan nomor sebelumnya menjadi previous (US-8)', () => {
  const view = deriveQueueView([
    served(1, '2026-10-17T09:00:00Z'),
    served(2, '2026-10-17T09:05:00Z'),
    ticket(3),
  ]);
  assert.equal(view.current?.ticket.id, 2);
  assert.equal(view.previous?.ticket.id, 1);
  assert.equal(view.next?.ticket.id, 3);
  assert.deepEqual(view.upcoming, []);
});

test('yang sedang dilayani ditentukan oleh completed_at terbaru, bukan id terkecil/terbesar', () => {
  const view = deriveQueueView([
    served(5, '2026-10-17T09:10:00Z'),
    served(3, '2026-10-17T09:20:00Z'),
  ]);
  assert.equal(view.current?.ticket.id, 3);
  assert.equal(view.previous?.ticket.id, 5);
});

test('setelah nomor terakhir dilayani dan tidak ada yang menunggu: next dan upcoming kosong (4.3 langkah 4)', () => {
  const view = deriveQueueView([served(1, '2026-10-17T09:00:00Z'), served(2, '2026-10-17T09:05:00Z')]);
  assert.equal(view.current?.ticket.id, 2);
  assert.equal(view.next, null);
  assert.deepEqual(view.upcoming, []);
});

test('tiket yang dilewati atau dibatalkan tidak tampil sebagai berikutnya maupun upcoming; dilewati masuk `skipped` (4.7, 4.9)', () => {
  const view = deriveQueueView([
    ticket(1, { status: 'skipped', skip_count: 1 }),
    ticket(2, { status: 'cancelled', cancelled_at: '2026-10-17T08:30:00Z' }),
    ticket(3),
    ticket(4),
  ]);
  assert.equal(view.next?.ticket.id, 3);
  assert.deepEqual(view.upcoming.map((e) => e.ticket.id), [4]);
  assert.equal(view.current, null);
  assert.deepEqual(view.skipped.map((e) => e.ticket.id), [1]);
});

test('sisipan di posisi 1 saat ada yang dilayani menjadi berikutnya, tidak menggantikan yang dilayani (4.12)', () => {
  const view = deriveQueueView([
    served(1, '2026-10-17T09:00:00Z'),
    ticket(2, { sort_key: 2 }),
    ticket(3, { sort_key: 0.5 }),
  ]);
  assert.equal(view.current?.ticket.id, 1);
  assert.equal(view.next?.ticket.id, 3);
  assert.deepEqual(view.upcoming.map((e) => e.ticket.id), [2]);
});

test('urutan menunggu mengikuti sort_key; sort_key sama diurutkan berdasarkan id', () => {
  const view = deriveQueueView([
    ticket(9, { sort_key: 5 }),
    ticket(4, { sort_key: 5 }),
    ticket(7, { sort_key: 1 }),
  ]);
  assert.deepEqual([view.next?.ticket.id, ...view.upcoming.map((e) => e.ticket.id)], [7, 4, 9]);
});

test('input tidak diubah urutannya', () => {
  const input = [ticket(3), ticket(1), ticket(2)];
  const snapshot = input.map((t) => t.id);
  deriveQueueView(input);
  assert.deepEqual(input.map((t) => t.id), snapshot);
});
