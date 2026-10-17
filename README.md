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
- Owner: buat antrian (perlu langganan aktif), ubah nama dan deskripsi, atur izin, panggil Next, tutup antrian.
- Customer: bergabung lewat tautan, melihat nomor yang sedang dilayani dan posisi di depan, serta riwayat tiket.
- Pembaruan real-time lewat Server-Sent Events.
- Mode terang dan gelap.

Yang belum tersedia (lihat [DESIGN.md](DESIGN.md#7-batasan-dan-yang-belum-dibangun)): skip, notifikasi,
pembatalan oleh customer, counter, admin per counter, QR dinamis, catatan, riwayat ekspor, login Google,
dan pembayaran langganan.

## Teknologi

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27, Gin, sqlx + pgx, JWT HS256, bcrypt |
| Database | PostgreSQL |
| Frontend | Expo SDK 57 (Expo Router, web static), React Native, NativeWind, Zustand |
| Deploy | nginx (reverse proxy), Docker (Dockerfile per layanan, belum di-build) |

## Struktur folder

```
backend/          API Go (cmd/server, internal/{handler,service,repository,dto,sse,...})
frontend/         Aplikasi Expo (src/app rute, src/components, src/hooks, src/lib, src/store), Dockerfile
db/               Dockerfile dan skema PostgreSQL (migrations/01_Schema.sql)
deploy/nginx/     Konfigurasi nginx untuk pemasangan di host, Dockerfile, dan template konfigurasi
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
```

### 2. Backend

```bash
cd backend
cp .env.example .env     # isi DATABASE_URL, JWT_SECRET, PORT, ALLOWED_ORIGINS
go run ./cmd/server
```

Backend berjalan di port dari `PORT` (default 8080 di konfigurasi proyek ini).

Variabel penting:

| Variabel | Isi |
|---|---|
| `DATABASE_URL` | DSN PostgreSQL, mis. `postgres://user:pass@localhost:5432/qms_db?sslmode=disable` |
| `JWT_SECRET` | Rahasia penandatanganan token. Ganti untuk produksi |
| `PORT` | Port HTTP backend |
| `ALLOWED_ORIGINS` | Origin frontend yang diizinkan (dipisah koma). Tidak perlu jika frontend dan API satu origin |

### 3. Frontend

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

# Unit frontend
cd frontend
npm run test:unit
```

End-to-end memakai Playwright di `frontend/playwright/` (`npm run e2e`). Tes ini butuh backend dan frontend yang berjalan.

## Docker (belum di-build)

Setiap layanan punya Dockerfile dan `Dockerfile.dockerignore` sendiri. Berkasnya sudah disiapkan, tetapi
belum pernah di-build dan belum ada `docker-compose.yml` yang menghubungkan keempatnya.

| Layanan | Dockerfile | Catatan |
|---|---|---|
| db | `db/Dockerfile` | `postgres:16-alpine`, skema dijalankan otomatis saat data directory kosong |
| backend | `backend/Dockerfile` | Build dua tahap, binary di `alpine`, user non-root |
| frontend | `frontend/Dockerfile` | Hanya menghasilkan folder `dist` untuk diisi ke volume, butuh build-arg `EXPO_PUBLIC_API_URL` |
| nginx | `deploy/nginx/Dockerfile` | Konfigurasi dibuat dari template saat start (`QMS_DOMAIN`, `BACKEND_HOST`, `FRONTEND_ROOT`), HTTP saja |

Kredensial (`POSTGRES_PASSWORD`, `DATABASE_URL`, `JWT_SECRET`) tidak dibakar ke image. Diisi saat container dijalankan.

## Build frontend untuk produksi

```bash
cd frontend
EXPO_PUBLIC_API_URL=https://domain-anda npx expo export --platform web --clear
```

Hasilnya ada di `frontend/dist/`. Salin ke folder yang dibaca nginx, lalu ikuti [deploy/nginx/README.md](deploy/nginx/README.md).

## Status

Proyek capstone, masih dalam pengembangan. Sebagian alur sudah terverifikasi dari dua perangkat lewat nginx
(join, Next, dan event SSE). Lihat [DESIGN.md](DESIGN.md) untuk batasan teknis, terutama hub SSE yang masih di memori satu proses.
