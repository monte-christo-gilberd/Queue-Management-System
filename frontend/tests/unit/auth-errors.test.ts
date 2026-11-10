/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { ApiError, apiFetch, setAuthToken, setOnUnauthorized } from '@/lib/api';
import { AUTH_UNAVAILABLE_MESSAGE, describeAuthError } from '@/lib/auth-errors';

test('auth-service mati: pesan ramah yang menyebut user yang sudah login tetap bisa memakai aplikasi', () => {
  for (const e of [
    new ApiError(503, 'AUTH_UNAVAILABLE', 'login is temporarily unavailable'),
    new ApiError(502, 'UNKNOWN', 'x'), // dari nginx tanpa body JSON
    new ApiError(504, 'UNKNOWN', 'x'),
  ]) {
    assert.equal(describeAuthError(e), AUTH_UNAVAILABLE_MESSAGE);
  }
  assert.match(AUTH_UNAVAILABLE_MESSAGE, /sudah masuk.*tetap bisa dipakai/);
});

test('galat login lain tidak berubah', () => {
  assert.equal(describeAuthError(new ApiError(401, 'UNAUTHORIZED', 'x')), 'Email/username atau password salah');
  assert.equal(describeAuthError(new ApiError(409, 'CONFLICT', 'x')), 'Email sudah terdaftar');
  assert.equal(describeAuthError(new ApiError(400, 'WEAK_PASSWORD', 'x')), 'Password minimal 8 karakter');
  assert.match(describeAuthError(new ApiError(500, 'INTERNAL_ERROR', 'x')), /kesalahan di server/);
  assert.equal(describeAuthError(new Error('boom')), 'Terjadi kesalahan, coba lagi');
});

/** Memasang fetch palsu yang menjawab status dan body JSON tertentu; mengembalikan fungsi pemulih. */
function stubFetch(status: number, body: unknown): () => void {
  const original = globalThis.fetch;
  globalThis.fetch = (async () => new Response(JSON.stringify(body), { status })) as typeof fetch;
  return () => {
    globalThis.fetch = original;
  };
}

test('503 AUTH_UNAVAILABLE TIDAK memicu logout, 401 dengan token tetap memicu', async () => {
  let logouts = 0;
  setOnUnauthorized(() => {
    logouts += 1;
  });
  setAuthToken('token-sudah-login');
  try {
    // auth-service mati: user yang sudah login tidak boleh dikeluarkan.
    let restore = stubFetch(503, { error: { code: 'AUTH_UNAVAILABLE', message: 'x' } });
    await assert.rejects(apiFetch('/api/v1/users/verify-email', { method: 'POST', body: {} }), (e: unknown) => {
      assert.ok(e instanceof ApiError);
      assert.equal(e.code, 'AUTH_UNAVAILABLE');
      return true;
    });
    restore();
    assert.equal(logouts, 0, '503 tidak boleh memanggil logout');

    // Token benar-benar ditolak: sesi habis, logout dipanggil.
    restore = stubFetch(401, { error: { code: 'UNAUTHORIZED', message: 'x' } });
    await assert.rejects(apiFetch('/api/v1/users/me'));
    restore();
    assert.equal(logouts, 1, '401 dengan token memicu logout');
  } finally {
    setAuthToken(null);
    setOnUnauthorized(null);
  }
});
