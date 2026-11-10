/**
 * display.ts: API pengaturan dan data layar publik antrian (papan nomor).
 *
 * Dipakai oleh : hooks/use-display-config.ts, hooks/use-display-screen.ts, components/DisplaySettingsCard.tsx,
 *                components/DisplayScreen.tsx, utils/display.ts (tipe saja).
 * Bergantung   : lib/api.ts (apiFetch, ApiError).
 * Catatan      : GET /api/v1/display/:token bersifat publik (tanpa login). Token membentuk URL, lihat display-url.ts.
 */
import { ApiError, apiFetch } from './api';

/** Konfigurasi layar publik sebuah antrian (owner). Token membentuk URL layar. */
export type DisplayConfig = {
  /** ID antrian yang memiliki layar ini. */
  queue_id: number;
  /** Token rahasia pembentuk URL layar publik; berubah setiap kali layar diputar (rotate). */
  token: string;
  /** Jumlah nomor "berikutnya" yang tampil, 0 sampai DISPLAY_MAX_COUNT. */
  upcoming_count: number;
  /** Jumlah nomor "dilewati" yang tampil, 0 sampai DISPLAY_MAX_COUNT. */
  skipped_count: number;
  /** Waktu layar diaktifkan (string ISO). */
  created_at: string;
  /** Waktu token terakhir diputar (string ISO); null bila belum pernah diputar. */
  rotated_at: string | null;
};

/** Satu kartu counter di layar publik. `serving` null = counter sedang tidak melayani. */
export type DisplayCounter = {
  /** ID counter. */
  id: number;
  /** Nama counter yang tampil di kartu. */
  name: string;
  /** Nomor yang sedang dilayani sebagai teks siap tampil; null bila counter tidak melayani. */
  serving: string | null;
};

/** Respons publik GET /api/v1/display/:token. Hanya teks siap tampil, tanpa id tiket atau user. */
export type DisplaySnapshot = {
  /** Nama antrian untuk judul layar. */
  queue_name: string;
  /** Deskripsi antrian; null bila kosong. */
  queue_desc: string | null;
  /** `active` bila antrian dibuka, `closed` bila sudah ditutup. */
  status: 'active' | 'closed';
  /** Lebih dari satu counter: nama counter ditampilkan di kartu. */
  multi_counter: boolean;
  /** Daftar counter beserta nomor yang sedang dilayani. */
  counters: DisplayCounter[];
  /** Nomor berikutnya yang akan dipanggil, sebagai teks siap tampil. */
  upcoming: string[];
  /** Nomor yang dilewati, sebagai teks siap tampil. */
  skipped: string[];
  /** Waktu snapshot dibuat di server (string ISO). */
  generated_at: string;
};

/** DISPLAY_MAX_COUNT: batas atas jumlah nomor "berikutnya" dan "dilewati". Sama dengan backend. */
export const DISPLAY_MAX_COUNT = 10;
/** DISPLAY_DEFAULT_UPCOMING: jumlah default nomor "berikutnya" yang tampil. */
export const DISPLAY_DEFAULT_UPCOMING = 5;

/** GET /api/v1/queues/:id/display — konfigurasi layar. null bila layar belum diaktifkan (404). */
export async function getDisplayConfig(queueId: number): Promise<DisplayConfig | null> {
  try {
    return await apiFetch<DisplayConfig>(`/api/v1/queues/${queueId}/display`);
  } catch (e) {
    if (e instanceof ApiError && e.status === 404) return null;
    throw e;
  }
}

/** PUT /api/v1/queues/:id/display: aktifkan layar atau ubah jumlahnya (token tidak berubah). Jumlah yang kosong tetap. */
export async function saveDisplayConfig(
  queueId: number,
  counts: { /** Jumlah nomor berikutnya; undefined berarti tidak diubah. */ upcoming_count?: number; /** Jumlah nomor dilewati; undefined berarti tidak diubah. */ skipped_count?: number } = {},
): Promise<DisplayConfig> {
  return apiFetch<DisplayConfig>(`/api/v1/queues/${queueId}/display`, { method: 'PUT', body: counts });
}

/** POST /api/v1/queues/:id/display/rotate: token baru; layar yang memakai tautan lama berhenti bekerja. */
export async function rotateDisplay(queueId: number): Promise<DisplayConfig> {
  return apiFetch<DisplayConfig>(`/api/v1/queues/${queueId}/display/rotate`, { method: 'POST' });
}

/** DELETE /api/v1/queues/:id/display: matikan layar. */
export async function disableDisplay(queueId: number): Promise<void> {
  await apiFetch<void>(`/api/v1/queues/${queueId}/display`, { method: 'DELETE' });
}

/** GET /api/v1/display/:token, tanpa login. 404 = tautan tidak berlaku (salah, diganti, atau dimatikan). */
export async function getPublicDisplay(token: string, signal?: AbortSignal): Promise<DisplaySnapshot> {
  return apiFetch<DisplaySnapshot>(`/api/v1/display/${encodeURIComponent(token)}`, { signal });
}

/**
 * Pesan untuk aksi pengaturan layar (owner). Fungsi murni.
 * Contoh: `BAD_REQUEST` menghasilkan "Jumlah harus antara 0 dan 10".
 */
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
