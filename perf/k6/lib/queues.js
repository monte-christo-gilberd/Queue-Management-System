// queues.js: langkah-langkah perjalanan pengguna (plan k6, bagian 4) sebagai fungsi kecil yang dipakai skenario.
import { get, post, put, json } from './api.js';
import { SEED_PASSWORD } from './config.js';

// joinTokenOf mengambil token join dari join_url (".../join?t=<token>").
function joinTokenOf(url) {
  const i = (url || '').indexOf('t=');
  return i < 0 ? '' : decodeURIComponent(url.slice(i + 2));
}

// fetchJoinToken: owner membaca QR statis antrian (dibuat lazy oleh backend lewat qr-service) dan mengembalikan token join.
export function fetchJoinToken(owner, queueId) {
  const res = get(`/api/v1/queues/${queueId}/qr`, owner.token, '/queues/:id/qr');
  const body = json(res);
  return { res, token: body && body.data ? joinTokenOf(body.data.join_url) : '' };
}

// enableDisplay: owner mengaktifkan layar publik dan mengembalikan token layarnya.
export function enableDisplay(owner, queueId) {
  const res = put(`/api/v1/queues/${queueId}/display`, owner.token, {}, '/queues/:id/display');
  const body = json(res);
  return { res, token: body && body.data ? body.data.token : '' };
}

// join: pelanggan bergabung lewat token QR. Bila sudah punya tiket, backend mengembalikan tiket yang ada (200).
export function join(customer, joinToken) {
  return post('/api/v1/join', customer.token, { t: joinToken }, '/join');
}

// status: pemegang tiket atau owner membaca status antrian. Ini sumber beban terbesar (dipanggil klien tiap event SSE).
export function status(account, queueId) {
  return get(`/api/v1/queues/${queueId}/status`, account.token, '/queues/:id/status');
}

// myTickets: daftar tiket milik pelanggan.
export function myTickets(customer) {
  return get('/api/v1/users/me/tickets', customer.token, '/users/me/tickets');
}

// listTickets: owner membaca daftar tiket antrian (maks 500 baris).
export function listTickets(owner, queueId) {
  return get(`/api/v1/queues/${queueId}/tickets`, owner.token, '/queues/:id/tickets');
}

// next, skip: owner memajukan counter default. Body kosong = counter default.
export function next(owner, queueId) {
  return post(`/api/v1/queues/${queueId}/next`, owner.token, {}, '/queues/:id/next');
}
export function skip(owner, queueId) {
  return post(`/api/v1/queues/${queueId}/skip`, owner.token, {}, '/queues/:id/skip');
}

// cancel: pelanggan membatalkan tiketnya sendiri.
export function cancel(customer, queueId) {
  return post(`/api/v1/queues/${queueId}/cancel`, customer.token, null, '/queues/:id/cancel');
}

// display: layar publik membaca snapshot (tanpa login).
export function display(displayToken) {
  return get(`/api/v1/display/${displayToken}`, '', '/display/:token');
}

// login: login nyata lewat auth-service (bcrypt). identifier = email akun seed yang punya password.
export function login(identifier) {
  return post('/api/v1/users/login', '', { identifier: identifier, password: SEED_PASSWORD }, '/users/login');
}
