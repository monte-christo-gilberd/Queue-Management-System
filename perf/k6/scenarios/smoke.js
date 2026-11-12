// smoke.js: uji asap (plan k6, bagian 4). Satu pengguna menjalankan seluruh perjalanan sekali untuk membuktikan skrip,
// data seed, kunci uji, dan semua endpoint benar SEBELUM beban dinaikkan. Semua check harus lulus (threshold checks==1).
//
//   make perf-smoke          (k6 dijalankan dari akar repo dengan BASE_URL dan PERF_TOKENS yang benar)
import { sleep } from 'k6';
import { assertSafeTarget, intEnv } from '../lib/config.js';
import { customers, owners, queues, ownerFor } from '../lib/data.js';
import * as q from '../lib/queues.js';
import { okStatus, hasData } from '../lib/checks.js';
import { build, latencyThresholds } from '../lib/summary.js';

// Endpoint yang dilaporkan latensinya di ringkasan (nama tag = templat URL).
const ENDPOINTS = ['POST /join', 'GET /queues/:id/status', 'GET /users/me/tickets', 'GET /queues/:id/tickets',
  'POST /queues/:id/next', 'POST /queues/:id/cancel', 'GET /display/:token', 'POST /users/login'];

// Pagar target: gagal sebelum request pertama bila BASE_URL bukan lingkungan uji.
assertSafeTarget();

export const options = {
  // p(99) tidak dihitung k6 secara bawaan; dibutuhkan untuk gerbang dan tabel ringkasan
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  // satu VU dan satu iterasi default; ITER mengulang perjalanan dengan pelanggan berbeda
  vus: 1,
  iterations: intEnv('ITER', 1),
  thresholds: {
    // semua check harus lulus: smoke yang "hampir benar" tidak berarti
    checks: ['rate==1'],
    http_req_failed: ['rate<0.01'],
    perf_http_429: ['count==0'],
    // pagar kasar latensi per endpoint (smoke satu pengguna: lebih dari 2 detik berarti ada yang salah)
    ...latencyThresholds(ENDPOINTS, 2000),
  },
};

// setup berjalan sekali: owner mengambil token join tiap antrian dan mengaktifkan layar publik, hasilnya dibagikan ke iterasi.
export function setup() {
  const prepared = queues.map((queue) => {
    const owner = ownerFor(queue);
    const jt = q.fetchJoinToken(owner, queue.id);
    okStatus(jt.res, 'setup qr');
    const d = q.enableDisplay(owner, queue.id);
    okStatus(d.res, 'setup display');
    return { queueId: queue.id, ownerId: owner.id, joinToken: jt.token, displayToken: d.token };
  });
  return { prepared: prepared };
}

// default: satu perjalanan lengkap. Pelanggan dan antrian dipilih dari indeks iterasi supaya iterasi berulang tidak bentrok.
export default function (data) {
  // iterasi ke-n memakai pelanggan ke-n dan antrian ke-(n mod jumlah antrian)
  const n = __ITER;
  const customer = customers[n % customers.length];
  const prep = data.prepared[n % data.prepared.length];
  const queue = queues.find((x) => x.id === prep.queueId);
  const owner = ownerFor(queue);

  // 0. kesehatan lewat nginx
  // (endpoint /health publik; dipanggil tanpa token)
  okStatus(q.display(prep.displayToken), 'layar publik');

  // 1. pelanggan bergabung lewat token QR
  const joined = q.join(customer, prep.joinToken);
  okStatus(joined, 'join');
  hasData(joined, 'join');

  // 2. pelanggan membaca status dan daftar tiketnya
  const st = q.status(customer, prep.queueId);
  okStatus(st, 'status pelanggan');
  hasData(st, 'status pelanggan');
  okStatus(q.myTickets(customer), 'tiket saya');

  // 3. owner membaca status dan daftar tiket, lalu memanggil nomor berikutnya
  okStatus(q.status(owner, prep.queueId), 'status owner');
  okStatus(q.listTickets(owner, prep.queueId), 'daftar tiket owner');
  // Next berhasil (200) bila ada yang menunggu; 409 QUEUE_EMPTY juga jawaban sah bila antrian kosong
  okStatus(q.next(owner, prep.queueId), 'next', [200, 409]);

  // 4. pelanggan membatalkan tiketnya (no-op aman bila sudah dilayani)
  okStatus(q.cancel(customer, prep.queueId), 'cancel');

  // 5. login nyata satu akun seed berpassword (mengukur bcrypt dan auth-service)
  const loginRes = q.login(`perf-owner-${(n % Math.min(owners.length, 10)) + 1}@perf.invalid`);
  okStatus(loginRes, 'login');

  sleep(0.2);
}

// handleSummary: teks ringkas dan berkas JSON laporan.
export function handleSummary(data) {
  return build(data, 'smoke', ENDPOINTS);
}
