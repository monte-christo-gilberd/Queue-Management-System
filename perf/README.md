# perf/: uji beban k6 menuju 1 juta pengguna aktif

Alat uji beban untuk QMS, mengikuti `documentation/docs/plan-k6-1-juta-user.md`. Folder ini mencakup **Fase 0 sampai 3** dari
rencana itu: prasyarat (pool database, nginx uji, tuning OS), kerangka k6 (smoke dan load), data seed massal dengan token JWT uji,
dan penahan koneksi SSE. Tangga beban sungguhan (Fase 4 dan seterusnya) belum dijalankan.

Semuanya berjalan di **stack terpisah** `qms-perf` (`docker-compose.perf.yml`): volume, jaringan (172.31.0.0/16), database, dan
kunci JWT sendiri, port host `127.0.0.1:18080`. Stack dev (`qms`, port 8080) dan prod (`qms-prod`) tidak disentuh.

```
perf/
  k6/lib/           config.js (BASE_URL dan pagar target), data.js, api.js, queues.js, checks.js, summary.js
  k6/scenarios/     smoke.js (uji asap), load.js (uji beban API)
  seed/seed.sql     data massal: owner, pelanggan, antrian, langganan aktif
  seed/mint/        program Go pembuat token JWT RS256 dengan kunci uji
  sse-holder/       program Go penahan koneksi SSE yang meniru klien
  tune-os.sh        tampilkan/terapkan setelan OS untuk banyak koneksi
  data/             tokens.json hasil seed (diabaikan git)
  reports/          ringkasan JSON tiap uji (diabaikan git)
  .generated/       server.perf.conf dan biner sse-holder (diabaikan git)
```

## Alur cepat

```bash
make perf-up                              # stack uji: db, qr-service, auth-service, backend, nginx uji
make perf-seed PERF_CUSTOMERS=5000        # data massal + perf/data/tokens.json
make perf-smoke                           # satu pengguna menjalankan seluruh perjalanan; semua check harus lulus
make perf-load PERF_USERS=1000 PERF_RPS=200 PERF_DURATION=2m
make perf-sse PERF_CONNS=5000 PERF_HOLD=2m   # tahan 5.000 koneksi SSE, owner menekan Next tiap 25 detik
make perf-down                            # hapus stack uji dan datanya (project qms-perf saja)
make perf-test                            # tes unit alat uji
```

Syarat: `.env` sudah ada (sama dengan stack dev), `docker`, `k6`, `go`, `curl`, `openssl`.
`PERF_CUSTOMERS` harus lebih besar atau sama dengan `PERF_USERS` dan `PERF_CONNS`.

## Yang berbeda dari stack dev (dan kenapa)

| Hal | Dev | Uji beban | Alasan |
|---|---|---|---|
| Image Go | `go run` dengan kode di-mount | tahap `prod` (binary statis) | Yang diukur aplikasi, bukan kompilator |
| nginx | `events {}` kosong = **1 worker, 512 koneksi** | `nginx.perf.conf`: `worker_processes auto`, `worker_connections 65536`, `backlog 8192` | Satu pengamat SSE memakai dua koneksi; bawaan membatasi sekitar 250 pengamat |
| `limit_req` | 1 sampai 20 req/s per IP | dilonggarkan (1.000.000 r/s) | Satu IP generator tidak boleh terkena 429 |
| Log akses nginx | JSON/teks per request | dimatikan | Tulis log per request membebani uji |
| Pool database | tanpa batas (sebelum Fase 0) | `DB_MAX_OPEN_CONNS` (bawaan 40 backend, 10 auth) | Postgres bawaan hanya sekitar 100 koneksi |
| Verifikasi email | menyala | mati (dan seed mengisi `email_verified_at`) | Akun seed tidak melewati alur email |
| Email dan push | mail-service, notify-service jalan | tidak dinyalakan; seed mematikan `email_notifications` | Uji tidak boleh mengirim email atau push |
| Kunci JWT | `keys-dev/` | `keys-perf/` (dibuat `make perf-keys`) | Kunci dev dan prod tidak dipakai di uji |

## Data uji

`seed.sql` mengisi database uji langsung lewat SQL (registrasi API memakai bcrypt dan terlalu lambat untuk ribuan akun):
- owner `perf-owner-<n>@perf.invalid` dan pelanggan `perf-customer-<n>@perf.invalid`, email terverifikasi, tiap owner berlangganan 365 hari;
- antrian `Perf <n>` dibagi rata ke owner, dengan counter default dan pengaturan bawaan;
- tabel `users` mewajibkan `password_hash` atau `google_id`, jadi semua akun memakai hash bcrypt yang sama (dihitung sekali, cost 10, password `perf-password`); sesi uji memakai JWT dan hanya bagian kecil lalu lintas yang login nyata.

`perf/seed/mint` membuat JWT RS256 (klaim `sub`, `iat`, `exp`, header `kid`) dengan `keys-perf/auth-private.pem`, persis seperti
token buatan auth-service, dan menulis `perf/data/tokens.json` (izin 0600, berlaku 24 jam). Backend uji memverifikasinya dengan
kunci publik pasangannya. Seed idempoten: bila data sudah ada, tidak ada yang diubah (`make perf-down` lalu `perf-up` untuk reset).

## Skenario k6

| Skenario | Isi | Gerbang (threshold) |
|---|---|---|
| `smoke.js` | Satu pengguna: layar publik, join, status, tiket saya, owner membaca status dan daftar tiket, Next, cancel, login nyata | semua check lulus, `http_req_failed < 1%`, nol 429 |
| `load.js` | `preload` pengguna aktif, lalu laju terbuka: 70% status, 15% join ulang, 5% daftar tiket owner, 5% layar publik, 2% login, Next tiap 25 detik per antrian | status p95 < 300 ms dan p99 < 800 ms, join p95 < 500 ms, galat < 0,5%, nol 429, nol `dropped_iterations` |

Status 409 (`QUEUE_EMPTY`, `QUEUE_CLOSED`) dianggap jawaban sah dan tidak dihitung sebagai galat. Tag `name` tiap request berupa
**templat** URL (`GET /queues/:id/status`), bukan URL mentah, supaya metrik tidak berkardinalitas tinggi.

Hasil tiap uji dicetak ke terminal dan ditulis ke `perf/reports/<skenario>-<waktu>.json`. Bila stack observability berjalan di
mesin yang sama, set `K6_PROMETHEUS_RW_SERVER_URL` (mis. `http://<ip-prometheus>:9090/api/v1/write`) agar `make perf-*` ikut
mengirim metrik k6 ke Prometheus (`-o experimental-prometheus-rw`). Prometheus tidak dipublikasikan ke host, jadi jalankan k6
dari jaringan yang sama atau lewat tunnel.

### Pagar pengaman target

`BASE_URL` (k6) dan `-base` (sse-holder) **ditolak** bila bukan jaringan privat (localhost, 127.x, 10.x, 172.16-31.x, 192.168.x,
`*.local`, `*.test`, `*.internal`), kecuali `ALLOW_PUBLIC_TARGET=1` / `-allow-public-target` untuk generator terdistribusi yang
menembak mesin uji lewat IP publik. Domain produksi (`montechristo.my.id`) dan host di `PERF_BLOCKED_HOSTS` (dipisah koma)
**selalu ditolak**, walau izin publik diberikan. Pemeriksaan terjadi sebelum satu request pun dikirim.

## Penahan koneksi SSE (`sse-holder`)

k6 memakai satu VU per koneksi, terlalu berat untuk puluhan ribu koneksi panjang. `sse-holder` menahan satu goroutine ringan
per koneksi. Langkahnya: owner membaca token join, pelanggan 0..N-1 bergabung (hanya pemegang tiket dan owner yang boleh
subscribe), koneksi `GET /queues/:id/event` dibuka dengan laju `-ramp` per detik, owner menekan Next tiap `-next-interval` per
antrian, dan (`-mimic`, bawaan nyala) setelah tiap event selain `queue_qr_updated` penahan memanggil `GET /status` setelah debounce
300 ms, **meniru frontend** (`use-queue-spot.ts`), yang justru sumber beban terbesar.

Yang dilaporkan: koneksi puncak dan yang bertahan di akhir, gagal dibuka, putus di tengah, event dan heartbeat, latensi buka
koneksi, latensi event (jam diterima dikurangi `ts` di event; akurat bila satu mesin atau jam disinkronkan), dan latensi serta galat
`GET /status` hasil peniruan. Kode keluar 3 bila koneksi yang bertahan di bawah 99,5% (gerbang plan).

```bash
make perf-sse PERF_CONNS=10000 PERF_HOLD=2m
perf/.generated/sse-holder -h                 # semua flag
```

## Batas OS

Tiap koneksi SSE memakai satu fd di penahan dan dua di nginx; satu IP sumber hanya punya sekitar 28 ribu port lokal ke satu
tujuan pada rentang bawaan Linux (32768-60999). `perf/tune-os.sh` menampilkan nilai sekarang dan yang disarankan (tanpa mengubah
apa pun); `sudo perf/tune-os.sh --apply` menerapkannya sampai reboot. Setelan ini untuk mesin **generator dan target uji**, bukan prod.
Untuk 1 juta koneksi dibutuhkan puluhan IP sumber (lihat rencana, bagian Lingkungan uji).

## Keterbatasan saat ini

- Fase 4 dan seterusnya (tangga 1 ribu sampai 1 juta, soak, spike, breakpoint) belum dijalankan.
- Branch ini belum punya stack observability, jadi gerbang yang bersumber dari metrik server (CPU database, memori per proses,
  `sse_clients_active`) belum otomatis; yang tersedia: metrik k6, laporan `sse-holder`, dan `docker stats`.
- Latensi event memakai jam server dan penahan; pada generator terpisah, sinkronkan jam (chrony/NTP).
- Hasil di mesin dev (8 CPU, 7,7 GB RAM) hanya orde besaran; angka resmi untuk 1 juta harus dari lingkungan terdistribusi.
