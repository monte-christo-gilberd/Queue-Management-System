# Queue Management System

Sistem antrian untuk layanan tatap muka. Owner membuat antrian dan memanggil nomor berikutnya. Customer bergabung
lewat tautan dan melihat posisi serta nomor yang sedang dilayani secara real-time, tanpa perlu refresh.

Dokumen lain:
- [DESIGN.md](docs/DESIGN.md): arsitektur, keputusan desain, dan batasan.
- [FLOW.md](docs/FLOW.md): alur pengguna dan diagram urutan.
- [api-contract.md](docs/api-contract.md): kontrak HTTP dan SSE.
- [deploy/nginx/README.md](deploy/nginx/README.md): pemasangan nginx.

## Fitur yang sudah berjalan

- Daftar, masuk, dan keluar. Masuk dengan email atau username.
- Owner: buat antrian (perlu langganan aktif), ubah nama dan deskripsi, panggil Next, tutup antrian.
- Owner: QR antrian statis/dinamis (token dibuat `qr-service`, gambar digambar di frontend dari `join_url`), penggantian kode statis, salin tautan join.
- Customer: bergabung lewat tautan/QR bertoken, melihat nomor yang sedang dilayani dan posisi di depan, serta riwayat tiket.
- Notifikasi giliran lewat Web Push (`notify-service`): nomor dipanggil → "Giliran Anda", nomor berikutnya → "Tinggal 1 nomor lagi". Nomor yang dilewati juga diberi tahu. Notifikasi diaktifkan otomatis saat join (tanpa efek bila izin ditolak). Owner bisa "Panggil ulang" (tombol menggantikan Skip di detail antrian, mati 3 detik setelah Next, sekali per nomor). Hanya di browser lewat HTTPS (`make tunnel`); Expo Go tidak mendukung.
- Pembaruan real-time lewat Server-Sent Events (termasuk `queue_qr_updated`).
- Mode terang dan gelap.

Yang belum tersedia (lihat [DESIGN.md](docs/DESIGN.md#7-batasan-dan-yang-belum-dibangun)): email,
pembatalan oleh customer, hapus nomor oleh owner, layar tampilan publik, counter kustom, catatan, ekspor riwayat, login Google,
dan pembayaran langganan.

## Teknologi

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27, Gin, sqlx + pgx, JWT HS256, bcrypt |
| qr-service | Go 1.27, token join HMAC (internal, tidak dipublikasikan nginx) |
| Database | PostgreSQL |
| Frontend | Expo SDK 57 (Expo Router, web static), React Native, NativeWind, Zustand |
| Deploy | Docker Compose, nginx (reverse proxy + rate limit login dan `/join`) |

## Struktur folder

```
backend/          API Go (cmd/server, internal/{handler,service,repository,dto,sse,qrclient,...})
qr-service/       Microservice QR (cmd/qr-service, internal/{config,joinlink,httpapi,qr}), Dockerfile
frontend/         Aplikasi Expo (src/app rute, src/components, src/hooks, src/lib, src/store), Dockerfile
db/               Dockerfile dan skema PostgreSQL (migrations/01_Schema.sql, 02_Indexes.sql)
deploy/nginx/     Konfigurasi nginx untuk pemasangan di host, Dockerfile, dan template konfigurasi
docker-compose.yml  Stack development (hot reload): db, qr-service, notify-service, backend, frontend; hanya nginx yang membuka port
docker-compose.prod.yml  Stack demo/produksi (target prod, nginx hanya di 127.0.0.1)
notify-service/   Web Push (LISTEN Postgres)
scripts/          backup-prod.sh (backup/restore DB prod)
.env.example      Template env dev, plus bagian per service untuk menjalankan di luar Docker
.env.prod.example Template env produksi
docs/             DESIGN.md, FLOW.md, api-contract.md, plan-live.md, plan-setelah-live.md, setup-domain.md, setup-waf-wifi.md
```

## Menjalankan secara lokal

### 1. Database

Buat database, lalu terapkan skema:

```bash
createdb qms_db
psql -d qms_db -f db/migrations/01_Schema.sql
psql -d qms_db -f db/migrations/02_Indexes.sql
```

### 2. qr-service

```bash
cd qr-service
cp ../.env.example .env     # salin bagian [qr-service/.env]; isi JOIN_LINK_SECRET (>=32 byte) + QR_SERVICE_TOKEN
go run ./cmd/qr-service  # http://localhost:8090
```

`JOIN_LINK_SECRET` hanya dipegang qr-service. Backend memanggil endpoint internal
`/internal/v1/join-token` dan `/internal/v1/join-token/verify` dengan header `X-Service-Token`.

### 3. Backend

```bash
cd backend
cp ../.env.example .env     # salin bagian [backend/.env]; isi DATABASE_URL, JWT_SECRET, PORT, ALLOWED_ORIGINS + variabel QR
go run ./cmd/server
```

Backend berjalan di port dari `PORT` (default 8080 di konfigurasi proyek ini).

Variabel penting:

| Variabel | Isi |
|---|---|
| `DATABASE_URL` | DSN PostgreSQL, mis. `postgres://user:pass@localhost:5432/qms_db?sslmode=disable` |
| `JWT_SECRET` | Rahasia penandatanganan token. Ganti untuk produksi |
| `PORT` | Port HTTP backend |
| `ALLOWED_ORIGINS` | Origin frontend yang diizinkan (dipisah koma) |
| `PUBLIC_APP_URL` | Origin frontend untuk `join_url`, mis. `http://localhost:8080` (lewat nginx) |
| `QR_SERVICE_URL` | Alamat qr-service, mis. `http://localhost:8090` |
| `QR_SERVICE_TOKEN` | Sama dengan `QR_SERVICE_TOKEN` milik qr-service |
| `QR_HTTP_TIMEOUT` | Timeout panggilan internal (detik, default 1) |

Catatan: `PUBLIC_APP_URL` harus bisa dibuka dari HP yang memindai QR (jangan `localhost` untuk uji perangkat).

### 4. Frontend

```bash
cd frontend
npm install
npm run web          # http://localhost:8081
npm run web:lan      # bisa diakses dari perangkat lain di jaringan yang sama
```

Alamat API diatur lewat `EXPO_PUBLIC_API_URL`. Kalau kosong, frontend memakai host yang dibuka dengan port 8080.

## Menjalankan tes

```bash
# Unit backend (tanpa database)
cd backend
go test ./internal/test/service_test/ ./internal/test/handler_test/ ./internal/test/router_test/ \
        ./internal/test/repository_test/ ./internal/dto_test/

# Integration backend (butuh DATABASE_URL yang bisa diakses; memakai schema sementara)
go test -tags integration ./internal/test/integration_test/ -count=1 -v

# qr-service
cd qr-service
go test ./...

# Unit frontend
cd frontend
npm run test:unit
```

End-to-end memakai Playwright di `frontend/playwright/` (`npm run e2e`). Tes ini butuh backend dan frontend yang berjalan.

## Docker Compose

```bash
cp .env.example .env     # isi secret (QMS_PUBLIC_ORIGIN = origin yang dibuka dari HP)
docker compose up --build
```

Buka `http://localhost:8080` (atau `QMS_PUBLIC_ORIGIN`). Hanya nginx yang membuka port ke host;
`qr-service` tidak punya `ports:` sehingga `/internal/v1/*` hanya bisa diakses dari jaringan Docker.

| Layanan | Dockerfile | Catatan |
|---|---|---|
| db | `db/Dockerfile` | `postgres:16-alpine`, skema dan index dijalankan otomatis saat data directory kosong |
| qr-service | `qr-service/Dockerfile` | Target `dev` (`go run`, restart manual), hanya di jaringan internal |
| notify-service | `notify-service/Dockerfile` | Target `dev`, hanya di jaringan internal. LISTEN ke Postgres (role `notify_ro`), kirim Web Push (VAPID) |
| backend | `backend/Dockerfile` | Target `dev` (`go run`, restart manual), healthcheck `/health`, hanya di jaringan internal |
| frontend | `frontend/Dockerfile` | Target `dev` (Metro, hot reload) di port 80 internal |
| nginx | `deploy/nginx/Dockerfile` | Reverse proxy, satu-satunya yang membuka port (80) |

Satu `docker-compose.yml` untuk development; semua kode di-mount. Frontend hot reload; Go perlu `docker compose restart backend` (`qr-service`, `notify-service`) setelah ubah kode. Untuk demo/produksi pakai `docker-compose.prod.yml` (lihat bawah).
Kredensial (`POSTGRES_PASSWORD`, `JWT_SECRET`, `JOIN_LINK_SECRET`, `QR_SERVICE_TOKEN`) tidak dibakar ke image.
Frontend memanggil API di origin halaman (`EXPO_PUBLIC_API_URL=origin`), jadi mengganti `QMS_PUBLIC_ORIGIN` tidak butuh build ulang frontend.

Untuk uji dari HP, jalankan `make up` (build + stack, alamat memakai IP LAN) atau `make tunnel` (HTTPS publik, kamera HP jalan); `make down` untuk berhenti. Perintah lain: `make restart` (backend, qr-service, notify-service), `make seed`, `make logs`. Lihat `Makefile` dan `deploy/nginx/README.md`.

## Demo/produksi di belakang Cloudflare Tunnel

```bash
cp .env.prod.example .env.prod && chmod 600 .env.prod   # isi rahasia (openssl rand -hex 32) dan kunci VAPID baru; .env dev tidak tersentuh
make down                   # stack dev memakai port 8080 yang sama
make prod-up                # docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
scripts/create-demo-accounts.sh   # 1 owner + 4 customer, sandi acak di ~/qms-demo-accounts.txt
make prod-guard             # setelah akun demo dibuat: seed/reset ditolak di DB ini
make prod-backup            # ~/qms-backups/qms-<waktu>.sql.gz; restore: scripts/backup-prod.sh restore <file>
make prod-reset             # kosongkan antrian/tiket/push untuk demo ulang; akun dipertahankan, backup otomatis dulu
```

Nginx hanya terikat ke `127.0.0.1:8080`; `cloudflared` di host menunjuk ke `http://localhost:8080`. IP klien dibaca dari `CF-Connecting-IP`. Backend dengan `APP_ENV=production` menolak start bila rahasia lemah atau origin bukan https. Database prod memakai volume sendiri tanpa seed. Backend berjalan satu instance (hub SSE di memori). Lihat `docs/setup-domain.md`, `docs/setup-waf-wifi.md`, dan `docs/plan-live.md`.

## Notifikasi (notify-service)

Variabel `.env` (lihat `.env.example`): `NOTIFY_DB_PASSWORD` (password role `notify_ro`, hanya baca), `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`. Buat kunci VAPID dengan `npx web-push generate-vapid-keys`.

Alur: trigger `tickets_notify_trg` di Postgres memancarkan `pg_notify('queue_events')` saat nomor dipanggil atau dipanggil ulang; `notify-service` mendengarkan, menghitung penerima, lalu mengirim push. Tanpa outbox, jadi push yang terjadi saat service mati hilang.

Catatan:
- Satu browser hanya melayani satu akun. Layar tiket mendaftarkan ulang langganan ke akun yang sedang login, dan logout melepas langganan.
- Push hanya jalan di HTTPS (`make tunnel`) dan di browser yang mendukung; Expo Go tidak.
- DB yang sudah berjalan tidak menjalankan ulang skrip init. Terapkan manual: `ALTER TABLE tickets ADD COLUMN recalled_at TIMESTAMPTZ;`, tabel `push_subscriptions`, fungsi `tickets_notify` dan trigger `tickets_notify_trg` (jika sudah ada versi lama tanpa event `skipped`, jalankan ulang `CREATE OR REPLACE FUNCTION` dan buat ulang trigger) (ada di akhir `db/migrations/01_Schema.sql`), serta role `notify_ro` (`db/migrations/03_notify_role.sh`). Pengaturan Skip juga butuh kolom `queue_settings.skip_fallback`, `skip_fallback_n`, `skip_max_returns`.
- Awalan nomor tiket: DB lama butuh `ALTER TABLE queues ADD COLUMN ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A' CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$');`, lalu restart backend dan notify-service (notify membaca kolom ini).
- Tes unit: `cd notify-service && go test ./...`.

## Build frontend untuk produksi

```bash
cd frontend
EXPO_PUBLIC_API_URL=https://domain-anda npx expo export --platform web --clear
```

Hasilnya ada di `frontend/dist/`. Salin ke folder yang dibaca nginx, lalu ikuti [deploy/nginx/README.md](deploy/nginx/README.md).

## Status

Proyek capstone, masih dalam pengembangan. Alur join berbasis token QR, QR statis/dinamis, penggantian kode,
Next, Skip dengan fallback, Recall, push notifikasi, dan event SSE sudah diimplementasikan. Rencana berikutnya ada di `docs/plan-setelah-live.md`. Lihat [docs/DESIGN.md](docs/DESIGN.md) untuk batasan teknis,
terutama hub SSE yang masih di memori satu proses.
