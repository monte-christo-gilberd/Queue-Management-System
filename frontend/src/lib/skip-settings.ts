import { apiFetch } from './api';
import type { SkipFallback, SkipSettings } from '@/types/api';

/** GET /api/v1/queues/:id/settings/skip */
export async function getSkipSettings(queueId: number): Promise<SkipSettings> {
  return apiFetch<SkipSettings>(`/api/v1/queues/${queueId}/settings/skip`);
}

/** PUT /api/v1/queues/:id/settings/skip */
export async function updateSkipSettings(queueId: number, input: SkipSettings): Promise<SkipSettings> {
  return apiFetch<SkipSettings>(`/api/v1/queues/${queueId}/settings/skip`, {
    method: 'PUT',
    body: { fallback: input.fallback, n: input.n ?? 0, max_returns: input.max_returns },
  });
}

export type SkipSettingsDraft = { fallback: SkipFallback; n: string; maxReturns: string };

function positiveInt(value: string): number | null {
  if (!/^\d+$/.test(value.trim())) return null;
  const n = Number(value);
  return n >= 1 ? n : null;
}

/** Validasi isian form. Mengembalikan pengaturan siap kirim, atau pesan galat. Fungsi murni. */
export function validateSkipSettings(
  draft: SkipSettingsDraft,
): { ok: true; value: SkipSettings } | { ok: false; error: string } {
  const max = positiveInt(draft.maxReturns);
  if (max === null) return { ok: false, error: 'Batas pengembalian harus angka 1 atau lebih' };
  if (draft.fallback === 'after_n') {
    const n = positiveInt(draft.n);
    if (n === null) return { ok: false, error: 'Jumlah nomor harus angka 1 atau lebih' };
    return { ok: true, value: { fallback: 'after_n', n, max_returns: max } };
  }
  return { ok: true, value: { fallback: draft.fallback, n: null, max_returns: max } };
}
