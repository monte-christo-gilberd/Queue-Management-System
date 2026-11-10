/**
 * skip-settings.ts: panggilan API dan validasi isian form pengaturan skip antrian (owner).
 *
 * Dipakai oleh : hooks/use-skip-settings.ts, components/SkipSettingsCard.tsx
 * Bergantung   : lib/api.ts (apiFetch), types/api.ts (SkipFallback, SkipSettings)
 * Catatan      : `n` hanya berarti untuk fallback `after_n`. Untuk fallback lain, validateSkipSettings menghasilkan n null,
 *                dan updateSkipSettings mengirimnya sebagai 0.
 */
import { apiFetch } from './api';
import type { SkipFallback, SkipSettings } from '@/types/api';

/** GET /api/v1/queues/:id/settings/skip — baca pengaturan skip antrian (owner). */
export async function getSkipSettings(queueId: number): Promise<SkipSettings> {
  return apiFetch<SkipSettings>(`/api/v1/queues/${queueId}/settings/skip`);
}

/** PUT /api/v1/queues/:id/settings/skip — ubah pengaturan skip antrian (owner). `n` yang kosong dikirim sebagai 0. */
export async function updateSkipSettings(queueId: number, input: SkipSettings): Promise<SkipSettings> {
  return apiFetch<SkipSettings>(`/api/v1/queues/${queueId}/settings/skip`, {
    method: 'PUT',
    body: { fallback: input.fallback, n: input.n ?? 0, max_returns: input.max_returns },
  });
}

/** SkipSettingsDraft: isian mentah dari form, masih berupa teks sebelum divalidasi. */
export type SkipSettingsDraft = {
  /** Pilihan fallback dari form. */
  fallback: SkipFallback;
  /** Isian jumlah nomor sebagai teks. Hanya dipakai bila fallback `after_n`. */
  n: string;
  /** Isian batas pengembalian sebagai teks. Wajib angka 1 atau lebih. */
  maxReturns: string;
};

/** positiveInt: mengubah teks menjadi bilangan bulat minimal 1. Mengembalikan null bila bukan angka bulat positif. */
function positiveInt(value: string): number | null {
  if (!/^\d+$/.test(value.trim())) return null;
  const n = Number(value);
  return n >= 1 ? n : null;
}

/**
 * validateSkipSettings: validasi isian form. Mengembalikan pengaturan siap kirim, atau pesan galat. Fungsi murni.
 * Batas pengembalian selalu diperiksa. Jumlah nomor hanya diperiksa bila fallback `after_n`.
 */
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
