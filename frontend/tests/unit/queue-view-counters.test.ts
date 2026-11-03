/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import type { Ticket } from '@/types/api';
import { deriveQueueView } from '@/utils/queue-view';

const ticket = (id: number, status: string, extra: Partial<Ticket> = {}): Ticket =>
  ({
    id,
    ticket_number: id,
    status,
    sort_key: id * 1000,
    called_counter_id: null,
    called_at: null,
    ...extra,
  }) as unknown as Ticket;

test('tanpa counterId, current = nomor yang paling baru dipanggil', () => {
  const view = deriveQueueView([
    ticket(1, 'currently_served', { called_counter_id: 1, called_at: '2026-01-01T10:00:00Z' }),
    ticket(2, 'currently_served', { called_counter_id: 2, called_at: '2026-01-01T10:05:00Z' }),
  ]);
  assert.equal(view.current?.ticket.id, 2);
  assert.deepEqual(view.serving.map((e) => e.ticket.id), [2, 1]);
});

test('dengan counterId, current = nomor milik counter itu saja', () => {
  const tickets = [
    ticket(1, 'currently_served', { called_counter_id: 1, called_at: '2026-01-01T10:00:00Z' }),
    ticket(2, 'currently_served', { called_counter_id: 2, called_at: '2026-01-01T10:05:00Z' }),
  ];
  assert.equal(deriveQueueView(tickets, 1).current?.ticket.id, 1);
  assert.equal(deriveQueueView(tickets, 2).current?.ticket.id, 2);
  assert.equal(deriveQueueView(tickets, 3).current, null, 'counter kosong');
  assert.equal(deriveQueueView(tickets, 3).serving.length, 2, 'serving tetap berisi semua counter');
});

test('waiting dan skipped tidak terpengaruh counterId', () => {
  const tickets = [
    ticket(1, 'currently_served', { called_counter_id: 1 }),
    ticket(2, 'waiting'),
    ticket(3, 'skipped'),
  ];
  const view = deriveQueueView(tickets, 2);
  assert.equal(view.next?.ticket.id, 2);
  assert.equal(view.skipped.length, 1);
});
