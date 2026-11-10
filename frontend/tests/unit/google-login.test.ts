/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError, setAuthToken, setOnUnauthorized } from '@/lib/api';
import { describeGoogleError, describeLinkError, AUTH_UNAVAILABLE_MESSAGE } from '@/lib/auth-errors';
import { clientIdFor, isGoogleEnabled } from '@/lib/google-auth';
import { confirmGoogleLink, exchangeGoogleIdToken, isLinkSessionOver } from '@/lib/google-login';

type Call = { url: string; method?: string; body?: unknown; auth?: string };

/** Memasang fetch palsu yang menjawab berurutan; merekam request. Mengembalikan catatan dan fungsi pemulih. */
function stubFetch(replies: Array<{ status: number; body: unknown }>): { calls: Call[]; restore: () => void } {
  const original = globalThis.fetch;
  const calls: Call[] = [];
  let i = 0;
  globalThis.fetch = (async (url: string, init?: RequestInit) => {
    const headers = (init?.headers ?? {}) as Record<string, string>;
    calls.push({
      url: String(url),
      method: init?.method,
      body: init?.body ? JSON.parse(String(init.body)) : undefined,
      auth: headers.Authorization,
    });
    const r = replies[Math.min(i++, replies.length - 1)];
    return new Response(JSON.stringify(r.body), { status: r.status });
  }) as typeof fetch;
  return {
    calls,
    restore: () => {
      globalThis.fetch = original;
    },
  };
}

// --- Konfigurasi client ID ---

test('tombol Google tampil hanya bila platform yang berjalan punya client ID', () => {
  const ids = { web: 'web-id', android: undefined, ios: '  ' };

  assert.equal(isGoogleEnabled('web', ids), true);
  assert.equal(isGoogleEnabled('android', ids), false, 'Android tidak dikonfigurasi');
  assert.equal(isGoogleEnabled('ios', ids), false, 'spasi saja dianggap kosong');
  assert.equal(isGoogleEnabled('windows', ids), false, 'platform tak dikenal');
  assert.equal(isGoogleEnabled('web', {}), false, 'tanpa konfigurasi sama sekali');
});

test('clientIdFor memilih client ID per platform dan membuang spasi', () => {
  const ids = { web: ' w ', android: 'a', ios: 'i' };

  assert.equal(clientIdFor('web', ids), 'w');
  assert.equal(clientIdFor('android', ids), 'a');
  assert.equal(clientIdFor('ios', ids), 'i');
  assert.equal(clientIdFor('macos', ids), undefined);
});

// --- Menukar ID token ---

test('ID token ditukar dengan token QMS lewat POST /users/google', async () => {
  const { calls, restore } = stubFetch([{ status: 200, body: { data: { token: 'jwt-qms' } } }]);
  try {
    const result = await exchangeGoogleIdToken('id-token-google');

    assert.deepEqual(result, { kind: 'token', token: 'jwt-qms' });
    assert.match(calls[0].url, /\/api\/v1\/users\/google$/);
    assert.equal(calls[0].method, 'POST');
    assert.deepEqual(calls[0].body, { id_token: 'id-token-google' });
  } finally {
    restore();
  }
});

test('409 GOOGLE_LINK_REQUIRED menjadi link_required (bukan galat) dan membawa link token', async () => {
  const { restore } = stubFetch([
    { status: 409, body: { error: { code: 'GOOGLE_LINK_REQUIRED', message: 'x', link_token: 'link-123' } } },
  ]);
  try {
    const result = await exchangeGoogleIdToken('id-token-google');

    assert.deepEqual(result, { kind: 'link_required', linkToken: 'link-123' });
  } finally {
    restore();
  }
});

test('GOOGLE_LINK_REQUIRED tanpa link_token dianggap galat (tidak ada yang bisa dikonfirmasi)', async () => {
  const { restore } = stubFetch([{ status: 409, body: { error: { code: 'GOOGLE_LINK_REQUIRED', message: 'x' } } }]);
  try {
    await assert.rejects(exchangeGoogleIdToken('t'), (e: unknown) => e instanceof ApiError && e.status === 409);
  } finally {
    restore();
  }
});

test('galat lain dari /users/google dilempar apa adanya', async () => {
  for (const [status, code] of [
    [401, 'UNAUTHORIZED'],
    [403, 'GOOGLE_EMAIL_UNVERIFIED'],
    [503, 'GOOGLE_UNAVAILABLE'],
    [409, 'CONFLICT'],
  ] as const) {
    const { restore } = stubFetch([{ status, body: { error: { code, message: 'x' } } }]);
    try {
      await assert.rejects(exchangeGoogleIdToken('t'), (e: unknown) => e instanceof ApiError && e.code === code && e.status === status);
    } finally {
      restore();
    }
  }
});

test('login Google gagal TIDAK memicu logout walau ada token lama (hanya 401 dengan token yang memicu)', async () => {
  let logouts = 0;
  setOnUnauthorized(() => {
    logouts += 1;
  });
  setAuthToken(null);
  const { restore } = stubFetch([{ status: 401, body: { error: { code: 'UNAUTHORIZED', message: 'x' } } }]);
  try {
    await assert.rejects(exchangeGoogleIdToken('token-buruk'));
    assert.equal(logouts, 0, 'tamu yang gagal login Google tidak memicu logout');
  } finally {
    restore();
    setOnUnauthorized(null);
  }
});

// --- Menautkan akun ---

test('penautan mengirim link token dan password, lalu mengembalikan token QMS', async () => {
  const { calls, restore } = stubFetch([{ status: 200, body: { data: { token: 'jwt-qms' } } }]);
  try {
    const token = await confirmGoogleLink('link-123', 'rahasia123');

    assert.equal(token, 'jwt-qms');
    assert.match(calls[0].url, /\/api\/v1\/users\/google\/link$/);
    assert.deepEqual(calls[0].body, { link_token: 'link-123', password: 'rahasia123' });
  } finally {
    restore();
  }
});

test('salah password dan sesi penautan berakhir dibedakan', async () => {
  const wrong = new ApiError(401, 'UNAUTHORIZED', 'x');
  const expired = new ApiError(400, 'INVALID_TOKEN', 'x');
  const taken = new ApiError(409, 'CONFLICT', 'x');

  assert.equal(isLinkSessionOver(wrong), false, 'salah password: boleh coba lagi dengan link token yang sama');
  assert.equal(isLinkSessionOver(expired), true);
  assert.equal(isLinkSessionOver(taken), true);
  assert.equal(isLinkSessionOver(new Error('x')), false);
  assert.equal(isLinkSessionOver(new ApiError(0, 'NETWORK_ERROR', 'x')), false, 'jaringan putus: boleh coba lagi');
});

// --- Pesan galat ---

test('pesan login Google untuk setiap galat', () => {
  assert.match(describeGoogleError(new ApiError(503, 'GOOGLE_UNAVAILABLE', 'x')), /Google sedang tidak tersedia/);
  assert.match(describeGoogleError(new ApiError(403, 'GOOGLE_EMAIL_UNVERIFIED', 'x')), /belum diverifikasi oleh Google/);
  assert.match(describeGoogleError(new ApiError(409, 'CONFLICT', 'x')), /tertaut ke akun Google yang lain/);
  assert.match(describeGoogleError(new ApiError(401, 'UNAUTHORIZED', 'x')), /Login dengan Google gagal/);
  assert.equal(describeGoogleError(new ApiError(0, 'NETWORK_ERROR', 'Tidak dapat terhubung ke server')), 'Tidak dapat terhubung ke server');
  assert.match(describeGoogleError(new ApiError(500, 'INTERNAL_ERROR', 'x')), /kesalahan di server/);
  assert.match(describeGoogleError(new Error('boom')), /Login dengan Google gagal/);
});

test('auth-service mati: login Google dan penautan memakai pesan yang sama dengan login biasa', () => {
  for (const e of [new ApiError(503, 'AUTH_UNAVAILABLE', 'x'), new ApiError(502, 'UNKNOWN', 'x'), new ApiError(504, 'UNKNOWN', 'x')]) {
    assert.equal(describeGoogleError(e), AUTH_UNAVAILABLE_MESSAGE);
    assert.equal(describeLinkError(e), AUTH_UNAVAILABLE_MESSAGE);
  }
});

test('pesan layar konfirmasi password', () => {
  assert.equal(describeLinkError(new ApiError(401, 'UNAUTHORIZED', 'x')), 'Password salah');
  assert.match(describeLinkError(new ApiError(400, 'INVALID_TOKEN', 'x')), /Sesi penautan habis.*login dengan Google lagi/);
  assert.match(describeLinkError(new ApiError(409, 'CONFLICT', 'x')), /tertaut ke akun Google yang lain/);
  assert.match(describeLinkError(new ApiError(400, 'BAD_REQUEST', 'x')), /Password wajib diisi/);
  assert.match(describeLinkError(new Error('boom')), /Terjadi kesalahan/);
});

test('ApiError menyimpan isi error backend (link_token) di details', () => {
  const e = new ApiError(409, 'GOOGLE_LINK_REQUIRED', 'm', { code: 'GOOGLE_LINK_REQUIRED', link_token: 't' });

  assert.equal(e.details?.link_token, 't');
  assert.equal(new ApiError(500, 'X', 'm').details, undefined, 'tanpa details tetap berfungsi seperti sebelumnya');
});
