/**
 * Klien HTTP untuk backend. Kontrak respons (lihat FRONTEND_GENERATION_GUIDE.md bagian 3):
 * sukses `{ data: T }`, gagal `{ error: { code, message } }`.
 *
 * Tidak mengimpor store. Token dan handler 401 dipasang dari luar (lihat store/authStore.ts),
 * sehingga tidak ada siklus impor antara api.ts dan store.
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

export const API_BASE_URL = resolveApiBase();

export class ApiError extends Error {
  /** Status HTTP. 0 = gagal terhubung ke server. */
  status: number;
  /** Kode dari backend (mis. `QUEUE_EMPTY`), atau `NETWORK_ERROR` / `UNKNOWN`. */
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

let authToken: string | null = null;
let onUnauthorized: (() => void) | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

/** Dipanggil saat backend membalas 401 untuk request yang membawa token (token kedaluwarsa atau tidak valid). */
export function setOnUnauthorized(handler: (() => void) | null) {
  onUnauthorized = handler;
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  body?: unknown;
  signal?: AbortSignal;
};

async function parseBody(res: Response): Promise<unknown> {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export function toApiError(status: number, body: unknown): ApiError {
  const err = (body as { error?: { code?: string; message?: string } } | null)?.error;
  return new ApiError(status, err?.code ?? 'UNKNOWN', err?.message ?? `Permintaan gagal (${status})`);
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
    if (e instanceof Error && e.name === 'AbortError') throw e;
    throw new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server');
  }

  const json = await parseBody(res);

  if (!res.ok) {
    if (res.status === 401 && authToken) onUnauthorized?.();
    throw toApiError(res.status, json);
  }

  return (json as { data: T }).data;
}
