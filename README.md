# Queue Management System

Sistem antrian untuk layanan tatap muka. Owner membuat antrian dan memanggil nomor berikutnya. Customer bergabung
lewat tautan dan melihat posisi serta nomor yang sedang dilayani secara real-time, tanpa perlu refresh.

Dokumen lain:
- [DESIGN.md](DESIGN.md): arsitektur, keputusan desain, dan batasan.
- [FLOW.md](FLOW.md): alur pengguna dan diagram urutan.
- [api-contract.md](api-contract.md): kontrak HTTP dan SSE.
- [deploy/nginx/README.md](deploy/nginx/README.md): pemasangan nginx.

## Fitur yang sudah berjalan

- Daftar, masuk, dan keluar. Masuk dengan email atau username.
- Owner: buat antrian (perlu langganan aktif), ubah nama dan deskripsi, panggil Next, tutup antrian.
- Owner: QR antrian statis/dinamis (token dibuat `qr-service`, gambar digambar di frontend dari `join_url`), penggantian kode statis, salin tautan join.
- Customer: bergabung lewat tautan/QR bertoken, melihat nomor yang sedang dilayani dan posisi di depan, serta riwayat tiket.
- Pembaruan real-time lewat Server-Sent Events (termasuk `queue_qr_updated`).
- Mode terang dan gelap.

Yang belum tersedia (lihat [DESIGN.md](DESIGN.md#7-batasan-dan-yang-belum-dibangun)): notifikasi/email,
pembatalan oleh customer, counter kustom, catatan, riwayat ekspor, login Google,
dan pembayaran langganan.

## Teknologi

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27, Gin, sqlx + pgx, JWT HS256, bcrypt |
| qr-service | Go 1.25, token join HMAC (internal, tidak dipublikasikan nginx) |
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
docker-compose.yml  Stack development (hot reload): db, qr-service, backend, frontend; hanya nginx yang membuka port
.env.example      Variabel untuk docker compose (secret, origin publik)
DESIGN.md         Desain sistem
FLOW.md           Alur pengguna
api-contract.md   Kontrak API
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
cp .env.example .env     # isi JOIN_LINK_SECRET (>=32 byte) + QR_SERVICE_TOKEN
go run ./cmd/qr-service  # http://localhost:8090
```

`JOIN_LINK_SECRET` hanya dipegang qr-service. Backend memanggil endpoint internal
`/internal/v1/join-token` dan `/internal/v1/join-token/verify` dengan header `X-Service-Token`.

### 3. Backend

```bash
cd backend
cp .env.example .env     # isi DATABASE_URL, JWT_SECRET, PORT, ALLOWED_ORIGINS + variabel QR
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
| backend | `backend/Dockerfile` | Target `dev` (`go run`, restart manual), healthcheck `/health`, hanya di jaringan internal |
| frontend | `frontend/Dockerfile` | Target `dev` (Metro, hot reload) di port 80 internal |
| nginx | `deploy/nginx/Dockerfile` | Reverse proxy, satu-satunya yang membuka port (80) |

Satu `docker-compose.yml` untuk development; semua kode di-mount. Frontend hot reload; Go perlu `docker compose restart backend` (atau `qr-service`) setelah ubah kode. Target `prod` tersedia di tiap Dockerfile untuk nanti.
Kredensial (`POSTGRES_PASSWORD`, `JWT_SECRET`, `JOIN_LINK_SECRET`, `QR_SERVICE_TOKEN`) tidak dibakar ke image.
Frontend memanggil API di origin halaman (`EXPO_PUBLIC_API_URL=origin`), jadi mengganti `QMS_PUBLIC_ORIGIN` tidak butuh build ulang frontend.

Untuk uji dari HP, jalankan `make up` (build + stack, alamat memakai IP LAN) atau `make tunnel` (HTTPS publik, kamera HP jalan); `make down` untuk berhenti. Perintah lain: `make restart` (backend dan qr-service), `make seed`, `make logs`. Lihat `Makefile` dan `deploy/nginx/README.md`.

## Build frontend untuk produksi

```bash
cd frontend
EXPO_PUBLIC_API_URL=https://domain-anda npx expo export --platform web --clear
```

Hasilnya ada di `frontend/dist/`. Salin ke folder yang dibaca nginx, lalu ikuti [deploy/nginx/README.md](deploy/nginx/README.md).

## Status

Proyek capstone, masih dalam pengembangan. Alur join berbasis token QR, QR statis/dinamis, penggantian kode,
Next, dan event SSE sudah diimplementasikan. Lihat [DESIGN.md](DESIGN.md) untuk batasan teknis,
terutama hub SSE yang masih di memori satu proses.
