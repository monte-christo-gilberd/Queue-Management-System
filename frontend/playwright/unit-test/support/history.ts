import { expect, type APIRequestContext } from '@playwright/test';

import {
  API_URL,
  closeQueueViaApi,
  createQueueViaApi,
  joinQueueViaApi,
  listTicketsViaApi,
  nextViaApi,
  registerAccount,
  skipViaApi,
  subscribedOwnerSession,
  uniqueName,
  type Account,
} from './index';

/** Helper bersama spec riwayat dan ekspor: antrian dengan semua jenis hasil akhir, lalu ditutup. */
export type HistoryQueue = {
  id: number; queue_name: string; closed_at: string;
  total: number; completed: number; skipped: number; cancelled: number; deleted: number; unserved: number;
};
export type HistoryTicket = {
  ticket_id: number; ticket_number: number; outcome: string; skip_count: number;
  customer_email: string; counter_name: string | null; note: string | null; completed_at: string | null;
};

export function auth(token: string) {
  return { Authorization: `Bearer ${token}` };
}

export async function ticketId(request: APIRequestContext, token: string, queueId: number, number: number): Promise<number> {
  const res = await listTicketsViaApi(request, token, queueId);
  const rows = (await res.json()).data.tickets as { id: number; ticket_number: number }[];
  const found = rows.find((t) => t.ticket_number === number);
  if (!found) throw new Error(`tiket ${number} tidak ada`);
  return found.id;
}

/**
 * Antrian dengan enam customer dan semua jenis hasil akhir, lalu ditutup:
 * A-001 dilewati, A-002 selesai, A-003 dilayani lalu selesai saat ditutup (punya catatan),
 * A-004 dibatalkan customer, A-005 dihapus owner, A-006 masih menunggu (belum dilayani).
 */
export async function closedQueueWithAllOutcomes(request: APIRequestContext, options: { note?: string } = {}) {
  const owner = await subscribedOwnerSession(request);
  if (!owner) throw new Error('owner berlangganan tidak tersedia');
  const name = uniqueName('Riwayat');
  const queue = await createQueueViaApi(request, owner.token, name);
  const customers: Account[] = [];
  for (let i = 0; i < 6; i++) {
    const c = await registerAccount(request, 'hist');
    await joinQueueViaApi(request, c.token, queue.id);
    customers.push(c);
  }
  expect((await nextViaApi(request, owner.token, queue.id)).status()).toBe(200); // A-001 dilayani
  expect((await skipViaApi(request, owner.token, queue.id)).status()).toBe(200); // A-001 dilewati, A-002 dilayani
  expect((await nextViaApi(request, owner.token, queue.id)).status()).toBe(200); // A-002 selesai, A-003 dilayani

  const cancel = await request.post(`${API_URL}/api/v1/queues/${queue.id}/cancel`, { headers: auth(customers[3].token) });
  expect(cancel.status()).toBe(200);
  const removeId = await ticketId(request, owner.token, queue.id, 5);
  const remove = await request.post(`${API_URL}/api/v1/queues/${queue.id}/tickets/${removeId}/remove`, { headers: auth(owner.token) });
  expect(remove.status()).toBe(200);
  const noteId = await ticketId(request, owner.token, queue.id, 3);
  const note = await request.put(`${API_URL}/api/v1/queues/${queue.id}/tickets/${noteId}/note`, {
    headers: auth(owner.token),
    data: { note: options.note ?? 'minta struk' },
  });
  expect(note.status()).toBeLessThan(300);

  const before = await historyList(request, owner.token);
  expect(before.queues.some((q) => q.id === queue.id)).toBe(false); // masih aktif: belum ada di riwayat

  await closeQueueViaApi(request, owner.token, queue.id); // helper sudah memeriksa 200
  return { owner, queue, name, customers };
}

export async function historyList(request: APIRequestContext, token: string, query = '') {
  const res = await request.get(`${API_URL}/api/v1/history/queues${query}`, { headers: auth(token) });
  expect(res.status()).toBe(200);
  return (await res.json()).data as { queues: HistoryQueue[]; next_cursor: string | null };
}
