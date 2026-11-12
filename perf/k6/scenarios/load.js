// load.js: uji beban (plan k6, bagian 4 dan 6). Beban target dihitung dari PERF_USERS (pengguna aktif U) dan PERF_RPS
// (total request per detik), dengan komposisi lalu lintas dari plan: 70% status, 15% join, 5% daftar tiket owner,
// 5% layar publik, 2% login, dan Next tiap 25 detik per antrian.
//
// CATATAN CAKUPAN: skenario ini menguji API (laju request). Koneksi SSE dalam jumlah besar ditahan oleh perf/sse-holder
// (k6 satu VU per koneksi tidak realistis); keduanya dijalankan bersamaan untuk beban penuh.
//
// Parameter (-e NAMA=nilai, Makefile `make perf-load` mengisinya):
//   PERF_USERS     pengguna aktif yang di-preload (bergabung ke antrian) sebelum beban, bawaan 1000
//   PERF_RPS       total request per detik saat beban penuh, bawaan 200
//   PERF_DURATION  lama fase beban penuh, bawaan 2m
//   PERF_VUS       VU yang dialokasikan awal untuk executor laju, bawaan 100 (maks 20x lipat)
//   PERF_PRELOAD_WAIT  jeda sebelum fase beban (menunggu preload selesai), bawaan dihitung dari PERF_USERS
import exec from 'k6/execution';
import { sleep } from 'k6';
import { assertSafeTarget, intEnv } from '../lib/config.js';
import { customers, queues, ownerFor } from '../lib/data.js';
import * as q from '../lib/queues.js';
import { okStatus } from '../lib/checks.js';
import { build, latencyThresholds, statusThresholds } from '../lib/summary.js';

// Endpoint yang dilaporkan latensinya di ringkasan (nama tag = templat URL).
const ENDPOINTS = ['GET /queues/:id/status', 'POST /join', 'GET /queues/:id/tickets', 'GET /display/:token',
  'POST /users/login', 'POST /queues/:id/next'];

assertSafeTarget();

// Jumlah pengguna aktif yang di-preload: dibatasi jumlah pelanggan di seed.
const USERS = Math.min(intEnv('PERF_USERS', 1000), customers.length);
const RPS = intEnv('PERF_RPS', 200);
const DURATION = __ENV.PERF_DURATION || '2m';
const VUS = intEnv('PERF_VUS', 100);
// Preload sekitar 200 join/detik per 50 VU; jeda = waktu itu + 10 detik cadangan.
const PRELOAD_WAIT = __ENV.PERF_PRELOAD_WAIT || `${Math.ceil(USERS / 200) + 10}s`;

// share menghitung laju (iterasi per detik) satu jenis lalu lintas dari total RPS; minimal 1.
const share = (fraction) => Math.max(1, Math.round(RPS * fraction));

// loadStages: naik 30 detik, tahan sepanjang DURATION, lalu turun 10 detik.
const loadStages = (rate) => [
  { target: rate, duration: '30s' },
  { target: rate, duration: DURATION },
  { target: 0, duration: '10s' },
];

// executor laju terbuka: request dikirim pada laju tetap walau server melambat (tidak menunggu respons, jadi antrean
// terlihat sebagai dropped_iterations dan latensi, bukan hilang diam-diam seperti pada model VU tertutup).
const arrival = (execName, rate) => ({
  executor: 'ramping-arrival-rate',
  exec: execName,
  startTime: PRELOAD_WAIT,
  startRate: 1,
  timeUnit: '1s',
  preAllocatedVUs: VUS,
  maxVUs: VUS * 20,
  stages: loadStages(rate),
});

export const options = {
  // p(99) tidak dihitung k6 secara bawaan; dibutuhkan untuk gerbang dan tabel ringkasan
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  scenarios: {
    // preload: setiap pengguna aktif bergabung ke antrian-nya (membuat tiket) sebelum beban dimulai
    preload: { executor: 'shared-iterations', exec: 'preload', vus: 50, iterations: USERS, maxDuration: '15m' },
    // status: sumber beban terbesar (klien memanggil ulang status setiap event SSE)
    status: arrival('status', share(0.70)),
    // join ulang: backend mengembalikan tiket yang ada; tetap memverifikasi token ke qr-service dan membaca database
    join: arrival('rejoin', share(0.15)),
    // owner membaca daftar tiket (dan riwayat), 5%
    owner_list: arrival('ownerList', share(0.05)),
    // layar publik, 5%
    display: arrival('displaySnapshot', share(0.05)),
    // login nyata (bcrypt), 2%
    auth: arrival('loginReal', share(0.02)),
    // Next: satu Next per antrian tiap 25 detik (counter menekan Next tiap 20 sampai 30 detik)
    owner_next: {
      executor: 'constant-arrival-rate', exec: 'ownerNext', startTime: PRELOAD_WAIT,
      rate: queues.length, timeUnit: '25s', duration: DURATION,
      preAllocatedVUs: 10, maxVUs: 100,
    },
  },
  // Gerbang uji (plan k6, bagian 6; angka awal, disetel setelah smoke dan tangga 1 ribu)
  thresholds: {
    'http_req_duration{name:GET /queues/:id/status}': ['p(95)<300', 'p(99)<800'],
    'http_req_duration{name:POST /join}': ['p(95)<500'],
    http_req_failed: ['rate<0.005'],
    perf_http_429: ['count==0'],
    // iterasi yang tidak sempat dijalankan karena VU habis: tanda generator atau server kewalahan
    dropped_iterations: ['count<1'],
    // endpoint lain: hanya pagar kasar 3 detik agar sub-metriknya tampil di ringkasan (gerbang sebenarnya di atas)
    ...latencyThresholds(ENDPOINTS.filter((n) => n !== 'GET /queues/:id/status' && n !== 'POST /join'), 3000),
    ...statusThresholds(),
    'http_reqs{name:GET /queues/:id/status}': ['count>=0'],
    'http_reqs{name:POST /join}': ['count>=0'],
  },
};

// setup: owner mengambil token join tiap antrian dan mengaktifkan layar publik; dibagikan ke semua skenario.
export function setup() {
  const prepared = queues.map((queue) => {
    const owner = ownerFor(queue);
    const jt = q.fetchJoinToken(owner, queue.id);
    okStatus(jt.res, 'setup qr');
    const d = q.enableDisplay(owner, queue.id);
    okStatus(d.res, 'setup display');
    return { queueId: queue.id, joinToken: jt.token, displayToken: d.token };
  });
  return { prepared: prepared };
}

// pickCustomer memilih pelanggan acak dari yang sudah di-preload (indeks 0 sampai USERS-1).
function pickCustomer() {
  const index = Math.floor(Math.random() * USERS);
  return { index: index, customer: customers[index], prep: null };
}

// prepOf memilih data antrian pelanggan (pembagian berputar, sama dengan preload).
function prepOf(data, index) {
  return data.prepared[index % data.prepared.length];
}

// preload: pelanggan ke-n (urutan iterasi global) bergabung ke antrian-nya.
export function preload(data) {
  const index = exec.scenario.iterationInTest;
  const prep = prepOf(data, index);
  okStatus(q.join(customers[index], prep.joinToken), 'preload join');
}

// status: pelanggan acak membaca status antrian-nya.
export function status(data) {
  const p = pickCustomer();
  okStatus(q.status(p.customer, prepOf(data, p.index).queueId), 'status');
}

// rejoin: pelanggan acak bergabung ulang (mengembalikan tiket yang sudah ada).
export function rejoin(data) {
  const p = pickCustomer();
  okStatus(q.join(p.customer, prepOf(data, p.index).joinToken), 'join ulang');
}

// ownerList: owner acak membaca daftar tiket antriannya.
export function ownerList(data) {
  const prep = data.prepared[Math.floor(Math.random() * data.prepared.length)];
  const queue = queues.find((x) => x.id === prep.queueId);
  okStatus(q.listTickets(ownerFor(queue), prep.queueId), 'daftar tiket owner');
}

// displaySnapshot: layar publik acak membaca snapshot.
export function displaySnapshot(data) {
  const prep = data.prepared[Math.floor(Math.random() * data.prepared.length)];
  okStatus(q.display(prep.displayToken), 'layar publik');
}

// loginReal: login nyata salah satu dari 10 owner seed berpassword.
export function loginReal() {
  okStatus(q.login(`perf-owner-${Math.floor(Math.random() * 10) + 1}@perf.invalid`), 'login');
}

// ownerNext: satu Next untuk antrian ke-(iterasi mod jumlah antrian). 409 QUEUE_EMPTY sah bila tidak ada yang menunggu.
export function ownerNext(data) {
  const prep = prepOf(data, exec.scenario.iterationInTest);
  const queue = queues.find((x) => x.id === prep.queueId);
  okStatus(q.next(ownerFor(queue), prep.queueId), 'next', [200, 409]);
  sleep(0.01);
}

// handleSummary: teks ringkas dan berkas JSON laporan.
export function handleSummary(data) {
  return build(data, `load-u${USERS}-rps${RPS}`, ENDPOINTS);
}
