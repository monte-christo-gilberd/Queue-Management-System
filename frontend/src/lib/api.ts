/**
 * api.ts: klien HTTP untuk backend antrian. Semua panggilan API di lib/ dan hooks/ lewat berkas ini.
 *
 * Dipakai oleh : lib/*.ts (queues, tickets, counters, users, payments, push, dll.), hooks/use-queue-events.ts
 * Bergantung   : fetch bawaan. Token dan handler 401 dipasang dari store/authStore.ts
 * Alur         : authStore memanggil setAuthToken setiap token berubah, dan setOnUnauthorized saat modul dimuat
 * Catatan      : Kontrak respons: sukses `{ data: T }`, gagal `{ error: { code, message } }`. Berkas ini tidak
 *                mengimpor store, supaya tidak ada siklus impor antara api.ts dan store.
 */

/**
 * Alamat backend. Urutan: EXPO_PUBLIC_API_URL (jika diisi), lalu di web host yang sedang dibuka
 * (localhost atau IP LAN, port 8080), lalu localhost. Nilai khusus `origin` (web di belakang nginx):
 * pakai origin halaman, jadi domain boleh berganti tanpa build ulang. Dengan begitu perangkat lain di jaringan
 * otomatis memanggil backend di mesin yang sama.
 */
function resolveApiBase(): string {
  const fromEnv = process.env.EXPO_PUBLIC_API_URL;
  if (fromEnv === 'origin' && typeof window !== 'undefined') return window.location.origin;
  if (fromEnv && fromEnv !== 'origin') return fromEnv;
  if (typeof window !== 'undefined' && window.location?.hostname) {
    return `${window.location.protocol}//${window.location.hostname}:8080`;
  }
  return 'http://localhost:8080';
}

/** API_BASE_URL: awalan URL semua request, dihitung sekali saat modul dimuat. */
export const API_BASE_URL = resolveApiBase();

/** ApiError: galat dari backend atau dari jaringan. Dilempar oleh apiFetch, apiUpload, dan apiDownload. */
export class ApiError extends Error {
  /** Status HTTP. 0 = gagal terhubung ke server. */
  status: number;
  /** Kode dari backend (mis. `QUEUE_EMPTY`), atau `NETWORK_ERROR` / `UNKNOWN`. */
  code: string;
  /** Isi objek `error` dari backend apa adanya, untuk galat yang membawa data tambahan (mis. `link_token` pada 409 `GOOGLE_LINK_REQUIRED`). */
  details?: Record<string, unknown>;

  constructor(status: number, code: string, message: string, details?: Record<string, unknown>) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

let authToken: string | null = null;
let onUnauthorized: (() => void) | null = null;

/** setAuthToken: menyimpan token untuk header Authorization pada request berikutnya. null menghapus header. */
export function setAuthToken(token: string | null) {
  authToken = token;
}

/** Dipanggil saat backend membalas 401 untuk request yang membawa token (token kedaluwarsa atau tidak valid). */
export function setOnUnauthorized(handler: (() => void) | null) {
  onUnauthorized = handler;
}

/** RequestOptions: opsi tambahan untuk apiFetch. */
type RequestOptions = {
  /** Metode HTTP. Default GET. */
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  /** Isi JSON. undefined berarti request tanpa body dan tanpa Content-Type. */
  body?: unknown;
  /** Sinyal pembatalan. Pembatalan oleh pemanggil diteruskan apa adanya, bukan diubah jadi ApiError. */
  signal?: AbortSignal;
};

/** Membaca body sebagai JSON. Body kosong atau bukan JSON menghasilkan null, supaya galat non-JSON tetap bisa dibaca. */
async function parseBody(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

/**
 * Mengunduh berkas (bukan JSON) dengan token auth. Kegagalan tetap dilempar sebagai `ApiError` dari body JSON
 * galat backend. Dipakai ekspor riwayat.
 */
export async function apiDownload(path: string): Promise<Blob> {
  const headers: Record<string, string> = {};
  if (authToken) headers.Authorization = `Bearer ${authToken}`;

  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, { headers });
  } catch {
    throw new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server');
  }
  if (!res.ok) {
    if (res.status === 401 && authToken) onUnauthorized?.();
    throw toApiError(res.status, await parseBody(res));
  }
  return res.blob();
}

/**
 * toApiError membuat ApiError dari body galat. Body yang tidak sesuai kontrak (mis. HTML dari nginx, atau null)
 * mendapat code `UNKNOWN` dan pesan `Permintaan gagal (status)`.
 */
export function toApiError(status: number, body: unknown): ApiError {
  const err = (body as { error?: { code?: string; message?: string } & Record<string, unknown> } | null)?.error;
  return new ApiError(status, err?.code ?? 'UNKNOWN', err?.message ?? `Permintaan gagal (${status})`, err ?? undefined);
}

/** Memanggil endpoint dan mengembalikan isi `data`. Selalu melempar `ApiError` untuk kegagalan. */
export async function apiFetch<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, signal } = options;

  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (authToken) headers.Authorization = `Bearer ${authToken}`;

  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    });
  } catch (e) {
    // Pembatalan dari pemanggil bukan galat jaringan, jadi tidak dibungkus.
    if (e instanceof Error && e.name === 'AbortError') throw e;
    throw new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server');
  }

  const json = await parseBody(res);

  if (!res.ok) {
    // 401 hanya dianggap sesi habis bila request membawa token. Login yang salah tidak memicu logout.
    if (res.status === 401 && authToken) onUnauthorized?.();
    throw toApiError(res.status, json);
  }

  // 204 No Content (mis. hapus foto profil): tidak ada body.
  if (res.status === 204) return undefined as T;

  return (json as { data: T }).data;
}

/**
 * Mengunggah form multipart (mis. foto profil) dengan token auth dan mengembalikan isi `data`.
 * Content-Type sengaja tidak diatur: fetch mengisinya lengkap dengan batas multipart.
 * Galat dari nginx (mis. 413 berbentuk HTML) tidak punya body JSON; kodenya tetap lewat `status`.
 */
export async function apiUpload<T>(path: string, form: FormData, method: 'PUT' | 'POST' = 'PUT'): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (authToken) headers.Authorization = `Bearer ${authToken}`;

  let res: Response;
  try {
    res = await fetch(`${API_BASE_URL}${path}`, { method, headers, body: form });
  } catch {
    throw new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server');
  }

  const json = await parseBody(res);
  if (!res.ok) {
    if (res.status === 401 && authToken) onUnauthorized?.();
    throw toApiError(res.status, json);
  }
  return (json as { data: T }).data;
}
