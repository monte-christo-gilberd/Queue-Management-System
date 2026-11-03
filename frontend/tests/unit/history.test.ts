/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { exportFileName, exportHistoryPath, historyQueuePath, historyQueuesPath } from '@/lib/history';
import type { HistoryQueue, HistoryTicket } from '@/types/api';
import { outcomeLabel, summarizeHistoryQueue, ticketTimeline } from '@/utils/history';

const queue = (over: Partial<HistoryQueue> = {}): HistoryQueue => ({
  id: 1,
  queue_name: 'Loket',
  ticket_prefix: 'A',
  opened_at: '2026-11-03T01:00:00Z',
  closed_at: '2026-11-03T09:00:00Z',
  total: 0,
  completed: 0,
  skipped: 0,
  cancelled: 0,
  deleted: 0,
  unserved: 0,
  ...over,
});

test('path daftar riwayat: tanpa parameter, dengan limit, dan dengan kursor yang di-encode', () => {
  assert.equal(historyQueuesPath(), '/api/v1/history/queues');
  assert.equal(historyQueuesPath(null, 10), '/api/v1/history/queues?limit=10');
  assert.equal(historyQueuesPath('123.4'), '/api/v1/history/queues?cursor=123.4');
  assert.equal(historyQueuesPath('a b/c'), '/api/v1/history/queues?cursor=a%20b%2Fc');
});

test('path detail riwayat memuat id antrian dan kursor', () => {
  assert.equal(historyQueuePath(7), '/api/v1/history/queues/7');
  assert.equal(historyQueuePath(7, '50', 25), '/api/v1/history/queues/7?limit=25&cursor=50');
});

test('ringkasan: hanya hasil yang lebih dari nol yang tampil', () => {
  assert.equal(summarizeHistoryQueue(queue({ total: 12, completed: 9, skipped: 2, cancelled: 1 })), '12 nomor · 9 selesai · 2 dilewati · 1 dibatalkan');
  assert.equal(summarizeHistoryQueue(queue({ total: 3, deleted: 1, unserved: 2 })), '3 nomor · 1 dihapus · 2 belum dilayani');
});

test('ringkasan: antrian tanpa nomor', () => {
  assert.equal(summarizeHistoryQueue(queue()), 'Tidak ada nomor');
});

test('label hasil akhir', () => {
  assert.equal(outcomeLabel('completed'), 'Selesai');
  assert.equal(outcomeLabel('skipped'), 'Dilewati');
  assert.equal(outcomeLabel('cancelled'), 'Dibatalkan');
  assert.equal(outcomeLabel('deleted'), 'Dihapus');
  assert.equal(outcomeLabel('unserved'), 'Belum dilayani');
});

test('baris waktu nomor: selalu ada bergabung; selesai dan dibatalkan hanya bila ada', () => {
  const base: HistoryTicket = {
    ticket_id: 1, ticket_number: 1, outcome: 'completed', skip_count: 0,
    joined_at: '2026-11-03T02:00:00Z', called_at: null, completed_at: null, cancelled_at: null,
    customer_name: 'a', customer_email: 'a@x', counter_name: null, note: null,
  };
  assert.equal(ticketTimeline(base).length, 1);
  assert.match(ticketTimeline(base)[0], /^Bergabung /);
  const done = ticketTimeline({ ...base, completed_at: '2026-11-03T03:00:00Z' });
  assert.equal(done.length, 2);
  assert.match(done[1], /^Selesai /);
  const cancelled = ticketTimeline({ ...base, outcome: 'cancelled', cancelled_at: '2026-11-03T03:00:00Z' });
  assert.match(cancelled[1], /^Dibatalkan /);
});

test('path ekspor memuat id antrian dan format', () => {
  assert.equal(exportHistoryPath(54, 'csv'), '/api/v1/history/queues/54/export?format=csv');
  assert.equal(exportHistoryPath(54, 'xlsx'), '/api/v1/history/queues/54/export?format=xlsx');
});

test('nama berkas ekspor sama dengan aturan backend', () => {
  assert.equal(exportFileName('Loket Utama!', 54, 'csv'), 'riwayat-loket-utama-54.csv');
  assert.equal(exportFileName('  Kasir  ', 7, 'xlsx'), 'riwayat-kasir-7.xlsx');
  assert.equal(exportFileName('日本語', 9, 'csv'), 'riwayat-antrian-9.csv');
  assert.equal(exportFileName('', 1, 'csv'), 'riwayat-antrian-1.csv');
  assert.ok(exportFileName('abc '.repeat(40), 3, 'csv').length <= 'riwayat-'.length + 40 + '-3.csv'.length);
  assert.doesNotMatch(exportFileName('../../etc/passwd', 1, 'csv'), /[/"\n;]/);
});
