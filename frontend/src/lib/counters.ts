import { apiFetch } from './api';

/** Nomor yang sedang dilayani sebuah counter. */
export type CounterServing = { ticket_id: number; ticket_number: number };

/** Counter antrian. `serving` hanya terisi di daftar counter (GET); respons buat dan ganti nama tidak memuatnya. */
export type Counter = {
  id: number;
  name: string;
  is_default: boolean;
  serving: CounterServing | null;
};

type ListCountersResponse = { action: 'list_counters'; queue_id: number; counters: Counter[] };

/** Batas jumlah counter per antrian, termasuk counter default (D3). Sama dengan backend. */
export const MAX_COUNTERS = 10;

/** Panjang maksimal nama counter dalam karakter (D3). Sama dengan backend. */
export const MAX_COUNTER_NAME_LENGTH = 30;

/** GET /api/v1/queues/:id/counters — daftar counter beserta nomor yang sedang dilayani (owner). */
export async function listCounters(queueId: number): Promise<Counter[]> {
  const data = await apiFetch<ListCountersResponse>(`/api/v1/queues/${queueId}/counters`);
  return data.counters;
}

/**
 * POST /api/v1/queues/:id/counters — buat counter. Tanpa nama, server memberi nama otomatis "Counter N".
 * Counter baru langsung dipakai perangkat yang membuatnya (seperti membuka tab baru).
 */
export async function createCounter(queueId: number, name?: string): Promise<Counter> {
  const body = name === undefined || name.trim() === '' ? {} : { name: name.trim() };
  return apiFetch<Counter>(`/api/v1/queues/${queueId}/counters`, { method: 'POST', body });
}

/**
 * Nama yang disarankan untuk counter baru: "Counter N" dengan N terkecil (mulai 2) yang belum dipakai.
 * Sama dengan nextCounterName di backend; hanya untuk mengisi form, server tetap memeriksa duplikat.
 */
export function suggestCounterName(existing: readonly string[]): string {
  for (let n = 2; ; n++) {
    const candidate = `Counter ${n}`;
    if (!existing.includes(candidate)) return candidate;
  }
}

/** PUT /api/v1/queues/:id/counters/:counter_id — ganti nama. Counter default boleh diganti namanya. */
export async function renameCounter(queueId: number, counterId: number, name: string): Promise<Counter> {
  return apiFetch<Counter>(`/api/v1/queues/${queueId}/counters/${counterId}`, {
    method: 'PUT',
    body: { name: name.trim() },
  });
}

/**
 * DELETE /api/v1/queues/:id/counters/:counter_id — hapus counter. Counter default tidak bisa dihapus (409).
 * Nomor yang sedang dilayani counter ini kembali ke posisi 1 urutan tunggu.
 */
export async function deleteCounter(queueId: number, counterId: number): Promise<void> {
  await apiFetch(`/api/v1/queues/${queueId}/counters/${counterId}`, { method: 'DELETE' });
}

/**
 * Pesan galat nama counter, atau null bila valid. Nama kosong hanya diterima untuk membuat counter
 * (nama otomatis), tidak untuk mengganti nama.
 */
export function validateCounterName(name: string, allowEmpty: boolean): string | null {
  const trimmed = name.trim();
  if (trimmed === '') return allowEmpty ? null : 'Nama counter wajib diisi';
  if ([...trimmed].length > MAX_COUNTER_NAME_LENGTH) return `Nama counter maksimal ${MAX_COUNTER_NAME_LENGTH} karakter`;
  return null;
}
