// config.js: konfigurasi bersama semua skenario k6 dan pagar pengaman target.
//
// Semua nilai bisa diubah lewat variabel lingkungan k6 (-e NAMA=nilai). Yang penting:
//   BASE_URL       alamat nginx uji (bawaan http://localhost:18080, project qms-perf)
//   PERF_TOKENS    path berkas tokens.json hasil `make perf-seed` (bawaan perf/data/tokens.json)
//   ALLOW_PUBLIC_TARGET=1   izinkan target di luar jaringan privat (JANGAN dipakai ke produksi)

// Alamat dasar target, tanpa garis miring di akhir.
export const BASE_URL = (__ENV.BASE_URL || 'http://localhost:18080').replace(/\/+$/, '');

// Path berkas token. open() di k6 dijalankan relatif terhadap berkas pemanggil, jadi Makefile memberi path absolut
// lewat PERF_TOKENS; nilai bawaan ini hanya untuk menjalankan `k6 run` langsung dari folder perf/k6/scenarios.
export const TOKENS_PATH = __ENV.PERF_TOKENS || '../../data/tokens.json';

// Password akun seed yang punya password nyata (seed.sql: auth_accounts akun pertama). Dipakai uji login.
export const SEED_PASSWORD = __ENV.PERF_SEED_PASSWORD || 'perf-password';

// Host yang SELALU ditolak walau ALLOW_PUBLIC_TARGET=1: domain produksi. Tambahkan lewat PERF_BLOCKED_HOSTS (koma).
const HARD_BLOCKED = ['montechristo.my.id'];

// hostFromUrl mengambil nama host dari URL (tanpa skema, userinfo, port, dan path).
function hostFromUrl(url) {
  const m = /^[a-z][a-z0-9+.-]*:\/\/(?:[^@/]*@)?(\[[^\]]+\]|[^:/?#]+)/i.exec(url);
  return m ? m[1].toLowerCase() : '';
}

// isPrivateHost: localhost, loopback, rentang privat RFC 1918, link-local, dan akhiran nama lokal.
function isPrivateHost(host) {
  if (host === 'localhost' || host === '[::1]') return true;
  if (/^127\./.test(host) || /^10\./.test(host) || /^192\.168\./.test(host) || /^169\.254\./.test(host)) return true;
  if (/^172\.(1[6-9]|2[0-9]|3[0-1])\./.test(host)) return true;
  return /\.(local|test|internal|localhost)$/.test(host);
}

// assertSafeTarget menghentikan uji bila BASE_URL bukan lingkungan uji. Dipanggil di init skenario, jadi k6 gagal
// sebelum satu request pun terkirim. Aturan: host produksi atau yang terdaftar di PERF_BLOCKED_HOSTS ditolak mutlak;
// host non-privat ditolak kecuali ALLOW_PUBLIC_TARGET=1 (untuk generator terdistribusi yang menembak mesin uji lewat IP publik).
export function assertSafeTarget() {
  const host = hostFromUrl(BASE_URL);
  const blocked = HARD_BLOCKED.concat((__ENV.PERF_BLOCKED_HOSTS || '').split(',').map((s) => s.trim().toLowerCase()).filter(Boolean));
  for (const b of blocked) {
    if (host === b || host.endsWith('.' + b)) {
      throw new Error(`BASE_URL ${BASE_URL} cocok dengan host terlarang (${b}): uji beban tidak boleh menembak produksi`);
    }
  }
  if (!isPrivateHost(host) && __ENV.ALLOW_PUBLIC_TARGET !== '1') {
    throw new Error(`BASE_URL ${BASE_URL} bukan jaringan privat. Bila ini memang mesin uji, set ALLOW_PUBLIC_TARGET=1`);
  }
}

// intEnv membaca bilangan bulat dari environment dengan nilai bawaan.
export function intEnv(name, def) {
  const v = parseInt(__ENV[name], 10);
  return Number.isFinite(v) && v > 0 ? v : def;
}
