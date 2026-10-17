/// <reference types="node" />
// Sumber: product-overview.md — US-15 / 4.4 (pelanggan melihat antrian yang masih relevan),
// 4.1 (antrian yang ditutup pindah ke riwayat), 4.9 / 4.10 (dibatalkan atau dilewati keluar dari urutan).
// Aturan pengguna: customer hanya melihat tiket yang masih menunggu di antrian aktif, atau nomornya sedang dilayani.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { isOnHome, type MyTicket } from '@/lib/tickets';

function myTicket(overrides: Partial<MyTicket> = {}): MyTicket {
  return {
    id: 1,
    queue_id: 10,
    ticket_number: 1,
    sort_key: 1,
    user_id: 100,
    status: 'waiting',
    skip_count: 0,
    last_notified_at: null,
    created_at: '2026-10-17T08:00:00Z',
    completed_at: null,
    cancelled_at: null,
    queue_name: 'Loket 1',
    queue_status: 'active',
    current_number: null,
    ...overrides,
  };
}

test('tiket yang masih menunggu di antrian aktif tampil di beranda (4.4)', () => {
  assert.equal(isOnHome(myTicket({ status: 'waiting', queue_status: 'active' })), true);
});

test('tiket menunggu di antrian yang sudah ditutup tidak tampil; masuk riwayat (4.1, 4.15)', () => {
  assert.equal(isOnHome(myTicket({ status: 'waiting', queue_status: 'closed' })), false);
});

test('nomor yang sedang dilayani tampil di beranda', () => {
  const ticket = myTicket({ status: 'completed', ticket_number: 3, current_number: 3, completed_at: '2026-10-17T09:00:00Z' });
  assert.equal(isOnHome(ticket), true);
});

test('nomor yang sudah lewat (ada nomor lain yang dilayani setelahnya) tidak tampil di beranda (US-15)', () => {
  const ticket = myTicket({ status: 'completed', ticket_number: 3, current_number: 4, completed_at: '2026-10-17T09:00:00Z' });
  assert.equal(isOnHome(ticket), false);
});

test('nomor selesai tanpa nomor yang sedang dilayani (current null) tidak tampil', () => {
  assert.equal(isOnHome(myTicket({ status: 'completed', ticket_number: 1, current_number: null })), false);
});

test('nomor yang sedang dilayani di antrian yang sudah ditutup tidak tampil', () => {
  const ticket = myTicket({ status: 'completed', ticket_number: 3, current_number: 3, queue_status: 'closed' });
  assert.equal(isOnHome(ticket), false);
});

test('tiket yang dibatalkan pelanggan tidak tampil (4.9: keluar dari urutan aktif)', () => {
  assert.equal(isOnHome(myTicket({ status: 'cancelled', cancelled_at: '2026-10-17T08:30:00Z' })), false);
});

test('tiket yang dilewati tidak tampil di beranda (4.10 / 4.7: bukan lagi menunggu)', () => {
  assert.equal(isOnHome(myTicket({ status: 'skipped', skip_count: 1 })), false);
});
