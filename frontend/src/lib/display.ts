import { ApiError, apiFetch } from './api';

/** Konfigurasi layar publik sebuah antrian (owner). Token membentuk URL layar. */
export type DisplayConfig = {
  queue_id: number;
  token: string;
  upcoming_count: number;
  skipped_count: number;
  created_at: string;
  rotated_at: string | null;
};

/** Satu kartu counter di layar publik. `serving` null = counter sedang tidak melayani. */
export type DisplayCounter = { id: number; name: string; serving: string | null };

/** Respons publik GET /api/v1/display/:token. Hanya teks siap tampil, tanpa id tiket atau user. */
export type DisplaySnapshot = {
  queue_name: string;
  queue_desc: string | null;
  status: 'active' | 'closed';
  /** Lebih dari satu counter: nama counter ditampilkan di kartu. */
  multi_counter: boolean;
  counters: DisplayCounter[];
  upcoming: string[];
  skipped: string[];
  generated_at: string;
};

/** Batas jumlah nomor "berikutnya" dan "dilewati". Sama dengan backend. */
export const DISPLAY_MAX_COUNT = 10;
export const DISPLAY_DEFAULT_UPCOMING = 5;

/** GET /queues/:id/display. null bila layar belum diaktifkan. */
export async function getDisplayConfig(queueId: number): Promise<DisplayConfig | null> {
  try {
    return await apiFetch<DisplayConfig>(`/api/v1/queues/${queueId}/display`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

/** PUT /queues/:id/display: aktifkan layar atau ubah jumlahnya (token tidak berubah). Jumlah yang kosong tetap. */
export async function saveDisplayConfig(
  queueId: number,
  counts: { upcoming_count?: number; skipped_count?: number } = {},
): Promise<DisplayConfig> {
  return apiFetch<DisplayConfig>(`/api/v1/queues/${queueId}/display`, { method: 'PUT', body: counts });
}

/** POST /queues/:id/display/rotate: token baru; layar yang memakai tautan lama berhenti bekerja. */
export async function rotateDisplay(queueId: number): Promise<DisplayConfig> {
  return apiFetch<DisplayConfig>(`/api/v1/queues/${queueId}/display/rotate`, { method: 'POST' });
}

/** DELETE /queues/:id/display: matikan layar. */
export async function disableDisplay(queueId: number): Promise<void> {
  await apiFetch<void>(`/api/v1/queues/${queueId}/display`, { method: 'DELETE' });
}

/** GET /display/:token, tanpa login. 404 = tautan tidak berlaku (salah, diganti, atau dimatikan). */
export async function getPublicDisplay(token: string, signal?: AbortSignal): Promise<DisplaySnapshot> {
  return apiFetch<DisplaySnapshot>(`/api/v1/display/${encodeURIComponent(token)}`, { signal });
}

/** Pesan untuk aksi pengaturan layar (owner). Fungsi murni. */
export function describeDisplayError(error: unknown): string {
  if (!(error instanceof ApiError)) return 'Terjadi kesalahan, coba lagi';
  switch (error.code) {
    case 'NETWORK_ERROR':
      return error.message;
    case 'NOT_FOUND':
      return 'Antrian tidak ditemukan';
    case 'BAD_REQUEST':
      return `Jumlah harus antara 0 dan ${DISPLAY_MAX_COUNT}`;
    default:
      return 'Terjadi kesalahan di server, coba lagi';
  }
}
