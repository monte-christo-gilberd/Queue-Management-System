/// <reference types="node" />
// Sumber: product-overview.md — NFR 4 (status dan layar publik diperbarui dalam beberapa detik tanpa refresh),
// 4.5 edge case (koneksi putus → status "menyambung ulang", bukan data lama). Backend mengirim `data: {json}`,
// komentar `: connected` / `: ping`, dan tipe event ada di dalam JSON.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  BACKOFF_MAX_MS,
  BACKOFF_START_MS,
  createSseParser,
  nextBackoff,
  parseQueueEvent,
  type SseMessage,
} from '@/lib/sse';

function collect() {
  const messages: SseMessage[] = [];
  const parser = createSseParser((m) => messages.push(m));
  return { messages, parser };
}

test('satu frame data menghasilkan satu pesan', () => {
  const { messages, parser } = collect();
  parser.push('data: {"type":"ticket_next","queue_id":3}\n\n');
  assert.deepEqual(messages, [{ event: undefined, data: '{"type":"ticket_next","queue_id":3}' }]);
});

test('potongan yang terpotong di tengah baris (termasuk CRLF) tetap menghasilkan pesan utuh', () => {
  const { messages, parser } = collect();
  parser.push('data: {"type":"ticket_');
  parser.push('joined","queue_id":');
  parser.push('3}\r');
  parser.push('\n\r\n');
  assert.equal(messages.length, 1);
  assert.equal(messages[0].data, '{"type":"ticket_joined","queue_id":3}');
});

test('komentar keep-alive (": connected", ": ping") tidak menghasilkan pesan', () => {
  const { messages, parser } = collect();
  parser.push(': connected\n\n: ping\n\n');
  assert.deepEqual(messages, []);
});

test('beberapa baris data dalam satu frame digabung dengan newline', () => {
  const { messages, parser } = collect();
  parser.push('data: line one\ndata: line two\n\n');
  assert.deepEqual(messages, [{ event: undefined, data: 'line one\nline two' }]);
});

test('dua frame dalam satu potongan menghasilkan dua pesan berurutan', () => {
  const { messages, parser } = collect();
  parser.push('data: {"a":1}\n\ndata: {"a":2}\n\n');
  assert.deepEqual(messages.map((m) => m.data), ['{"a":1}', '{"a":2}']);
});

test('field event dibawa ke pesan dan tidak terbawa ke frame berikutnya', () => {
  const { messages, parser } = collect();
  parser.push('event: queue_updated\ndata: x\n\ndata: y\n\n');
  assert.deepEqual(messages, [
    { event: 'queue_updated', data: 'x' },
    { event: undefined, data: 'y' },
  ]);
});

test('frame tanpa baris data tidak menghasilkan pesan', () => {
  const { messages, parser } = collect();
  parser.push('event: ping\n\n');
  assert.deepEqual(messages, []);
});

test('format field tanpa spasi setelah titik dua tetap dibaca', () => {
  const { messages, parser } = collect();
  parser.push('data:{"type":"queue_closed","queue_id":1}\n\n');
  assert.equal(messages[0]?.data, '{"type":"queue_closed","queue_id":1}');
});

test('event antrian yang dikenal diubah menjadi QueueEvent (NFR 4)', () => {
  const event = parseQueueEvent('{"type":"ticket_next","queue_id":3,"ts":"2026-10-17T09:00:00Z"}');
  assert.deepEqual(event, { type: 'ticket_next', queue_id: 3, payload: undefined, ts: '2026-10-17T09:00:00Z' });
});

test('payload dibawa apa adanya; ts opsional', () => {
  const event = parseQueueEvent('{"type":"queue_updated","queue_id":3,"payload":{"name":"Loket"}}');
  assert.deepEqual(event, { type: 'queue_updated', queue_id: 3, payload: { name: 'Loket' }, ts: undefined });
});

test('JSON rusak, bukan objek, atau tipe tidak dikenal ditolak', () => {
  assert.equal(parseQueueEvent('{bukan json'), null);
  assert.equal(parseQueueEvent('null'), null);
  assert.equal(parseQueueEvent('"teks"'), null);
  assert.equal(parseQueueEvent('{"type":"ticket_skipped","queue_id":3}'), null);
  assert.equal(parseQueueEvent('{"queue_id":3}'), null);
});

test('queue_id harus bilangan bulat', () => {
  assert.equal(parseQueueEvent('{"type":"ticket_next"}'), null);
  assert.equal(parseQueueEvent('{"type":"ticket_next","queue_id":"3"}'), null);
  assert.equal(parseQueueEvent('{"type":"ticket_next","queue_id":3.5}'), null);
});

test('semua jenis event dari backend dikenali', () => {
  for (const type of ['ticket_joined', 'ticket_next', 'queue_updated', 'queue_closed', 'queue_qr_updated']) {
    assert.equal(parseQueueEvent(`{"type":"${type}","queue_id":1}`)?.type, type);
  }
});

test('backoff reconnect menggandakan jeda dan dibatasi maksimum', () => {
  assert.equal(BACKOFF_START_MS, 1000);
  assert.equal(nextBackoff(BACKOFF_START_MS), 2000);
  assert.equal(nextBackoff(2000), 4000);
  assert.equal(nextBackoff(20000), BACKOFF_MAX_MS);
  assert.equal(nextBackoff(BACKOFF_MAX_MS), BACKOFF_MAX_MS);
});

test('backoff awal yang lebih kecil dari jeda awal dinaikkan dulu ke jeda awal', () => {
  assert.equal(nextBackoff(0), 2000);
});
