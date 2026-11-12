// api.js: pembungkus HTTP tipis untuk API QMS. Semua request diberi tag `name` berupa TEMPLAT URL (mis.
// "GET /queues/:id/status"), bukan URL mentah, supaya metrik k6 tidak berkardinalitas tinggi (plan k6, bagian Risiko).
import http from 'k6/http';
import { Counter } from 'k6/metrics';
import { BASE_URL } from './config.js';

// Status yang dianggap WAJAR oleh http_req_failed: semua 2xx dan 3xx, ditambah 409 (jawaban sah backend, mis. QUEUE_EMPTY
// saat Next pada antrian kosong, atau QUEUE_CLOSED). Tanpa ini 409 dihitung gagal dan merusak gerbang tingkat galat.
// Dipanggil di init, jadi berlaku untuk semua request skenario yang mengimpor modul ini.
http.setResponseCallback(http.expectedStatuses({ min: 200, max: 399 }, 409));

// Penghitung respons 429 dari limit_req nginx. Gerbang uji: harus 0 (zona dilonggarkan di nginx.perf.conf).
export const rate429 = new Counter('perf_http_429');

// headers membuat header JSON, plus Authorization bila ada token.
function headers(token) {
  const h = { 'Content-Type': 'application/json' };
  if (token) h.Authorization = 'Bearer ' + token;
  return h;
}

// note menghitung 429 dan mengembalikan respons apa adanya.
function note(res) {
  if (res.status === 429) rate429.add(1);
  return res;
}

// get: GET dengan token opsional; name = templat untuk tag.
export function get(path, token, name) {
  return note(http.get(BASE_URL + path, { headers: headers(token), tags: { name: 'GET ' + name } }));
}

// post: POST dengan body JSON (objek atau null) dan token opsional.
export function post(path, token, body, name) {
  const payload = body === null || body === undefined ? null : JSON.stringify(body);
  return note(http.post(BASE_URL + path, payload, { headers: headers(token), tags: { name: 'POST ' + name } }));
}

// put: PUT dengan body JSON.
export function put(path, token, body, name) {
  return note(http.put(BASE_URL + path, JSON.stringify(body || {}), { headers: headers(token), tags: { name: 'PUT ' + name } }));
}

// json mengurai body respons; mengembalikan null bila bukan JSON (supaya check tidak melempar galat).
export function json(res) {
  try {
    return res.json();
  } catch (e) {
    return null;
  }
}
