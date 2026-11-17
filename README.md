# Queue Management System (Sistem Antrian)

Sistem antrian untuk layanan tatap muka. Owner membuat antrian dan memanggil nomor berikutnya. Customer bergabung
lewat tautan dan melihat posisi serta nomor yang sedang dilayani secara real-time, tanpa perlu refresh.

Isi dokumen:

1. [Overview and assumptions](#1-overview-and-assumptions): apa ini, dan apa yang diasumsikan.
2. [Setup and run](#2-setup-and-run): cara menjalankannya.
3. [What was built](#3-what-was-built): apa saja yang ada.
4. [What was deferred and why](#4-what-was-deferred-and-why): apa yang belum ada, dan apakah itu pilihan.
5. [Architecture](#5-architecture): bentuk sistemnya.
6. [Trade-offs](#6-trade-offs): mengapa berbentuk begitu, dan apa harganya.
7. [Next steps](#7-next-steps): apa yang dikerjakan bila ada waktu lebih.

Dokumentasi teknis lengkap per komponen (arsitektur, tech stack, keputusan desain, penjelasan kode backend, tiap service, database, frontend, deployment, observability, dan testing) ada di [docs/dokumentasi-teknis/README.md](../docs/dokumentasi-teknis/README.md). Bila ada beda antara dokumen ini dan kode, kode yang benar.

## 1. Overview and assumptions

### Alur pengguna

**Owner**
1. Daftar, verifikasi email lewat tautan yang dikirim (lihat catatan di bawah), lalu buat antrian baru. Membuat antrian butuh email terverifikasi dan langganan aktif (checkout langganan juga butuh email terverifikasi).
2. Di detail antrian, bagikan QR atau tautan join ke customer. QR statis berisi kode tetap yang bisa diganti; QR dinamis kodenya berubah berkala.
3. Di Dashboard, tekan **Next** untuk memanggil nomor berikutnya. **Skip** melewati nomor yang sedang dilayani, dan **Panggil ulang** mengulang panggilan.
4. Tutup antrian saat layanan selesai. Customer yang belum bergabung tidak bisa join lagi.

**Customer**
1. Daftar dan verifikasi email (banner di halaman Profil, ada tombol kirim ulang). Scan QR atau buka tautan join. Login dulu bila belum masuk, lalu kembali ke halaman join.
2. Tekan Gabung untuk mendapat nomor antrian. Scan berulang tidak membuat tiket kedua.
3. Pantau posisi dan nomor yang sedang dilayani di Beranda atau Antrian Saya. Notifikasi "Giliran Anda" datang saat nomor dipanggil.

Verifikasi email: login tetap bisa sebelum diverifikasi, tetapi bergabung ke antrian (nomor baru) dan membuat antrian ditolak (403 `EMAIL_NOT_VERIFIED`) sampai tautan di email dibuka. Di dev email masuk ke Mailpit; di produksi ikut `MAIL_MODE` ([bagian 2](#verifikasi-email)).

Diagram dan detail lengkap ada di [FLOW.md](docs/FLOW.md).

### Asumsi

- Layanan tatap muka: customer hadir langsung dan membawa HP. Owner memanggil nomor satu per satu.
- Customer wajib login dan memverifikasi email sebelum bergabung. Tidak ada join anonim. Verifikasi hanya membuktikan kepemilikan email lewat tautan; domain email tidak diperiksa (tanpa cek MX atau allowlist).
- Join hanya lewat kode join 6 karakter dari QR atau tautan (dibuat backend). Kode statis bisa diganti owner; kode dinamis berganti setelah ada yang join dan kedaluwarsa setelah 120 detik.
- Owner butuh langganan aktif untuk membuat antrian. User biasa menjadi owner lewat pembayaran QRIS BorderPay untuk 1 bulan (30 hari).
- Layar publik hanya menampilkan nomor, tanpa nama customer (privasi).
- Riwayat dan antrian tidak pernah dihapus permanen (keputusan produk 31 Okt 2026).
- Backend berjalan satu instance, karena hub SSE ada di memori proses.
- Frontend dan API satu origin lewat nginx. `PUBLIC_APP_URL` harus bisa dibuka dari HP yang memindai QR.
- Web Push butuh HTTPS dan browser yang mendukung. Notifikasi di aplikasi Android memakai FCM dan butuh APK hasil EAS Build (tidak jalan di Expo Go).

### Status

Proyek capstone. Hasil pembacaan seluruh kode: backend, frontend, dan keempat service pendukung (auth, notify, mail, payment) sudah terhubung ujung ke ujung, tanpa stub dan tanpa TODO di backend maupun service. Login Google tersambung lewat auth-service dan menyala hanya bila `GOOGLE_CLIENT_IDS` diisi. Sisanya adalah ketahanan dan operasional (lihat [bagian 4](#4-what-was-deferred-and-why) dan [bagian 7](#7-next-steps)).

### Dokumen lain

- [docs/dokumentasi-teknis/README.md](../docs/dokumentasi-teknis/README.md): dokumentasi teknis lengkap hasil pembacaan kode (14 dokumen: arsitektur, tech stack, ADR, backend, auth, notify, mail, payment, database, frontend, deployment, observability, testing).
- [DESIGN.md](docs/DESIGN.md): arsitektur, keputusan desain, dan batasan.
- [FLOW.md](docs/FLOW.md): alur pengguna dan diagram urutan.
- [api-contract.md](docs/api-contract.md): kontrak HTTP dan SSE.
- [panduan-sse-backend.md](docs/panduan-sse-backend.md): panduan package `sse` di backend untuk yang belum familiar dengan Server-Sent Events: konsep, alur, cara mengirim event baru, dan cara mencobanya.
- [deploy/nginx/README.md](deploy/nginx/README.md): pemasangan nginx.

## 2. Setup and run

Dua jalur: **lokal** (tiap service dijalankan manual) atau **Docker Compose** (seluruh stack sekaligus, lebih mudah). Bagian di bawah berurutan: lokal, tes, Docker Compose, demo/produksi, build frontend, lalu konfigurasi layanan pendukung.

### Menjalankan secara lokal

#### 1. Database

Buat database, lalu terapkan skema (role per service dibuat oleh skrip `db/migrations/*.sh`, otomatis hanya di Docker):

```bash
createdb qms_db
for f in db/migrations/01*.sql db/migrations/02_Indexes.sql; do psql -d qms_db -v ON_ERROR_STOP=1 -f "$f"; done
```

Skema dipecah per domain (`01a_accounts.sql` sampai `01h_role_grants.sql`), lalu `02_Indexes.sql`; urutan nama = urutan ketergantungan. Peta file dan aturan menambah atau mengubah skema ada di [db/migrations/README.md](db/migrations/README.md).

#### 2. Kunci token login

```bash
scripts/gen-auth-keys.sh   # membuat keys/auth-private.pem dan keys/auth-public.pem (idempoten, tidak ikut git)
```

Token login ditandatangani auth-service dengan kunci privat (RS256) dan diverifikasi backend secara lokal dengan kunci publik.

#### 3. auth-service

```bash
cd auth-service
# Env: DATABASE_URL (role auth_rw), AUTH_JWT_PRIVATE_KEY_FILE, PUBLIC_APP_URL, MAIL_SERVICE_URL + MAIL_SERVICE_TOKEN (opsional), GOOGLE_CLIENT_IDS (opsional)
go run ./cmd/auth-service  # http://localhost:8094
```

Melayani login, registrasi, login Google, reset password, dan verifikasi email. Konfigurasi dibaca dari environment; `../.env` dimuat bila ada.

#### 4. Backend

```bash
cd backend
# Env: DATABASE_URL, JWT_PUBLIC_KEY_FILE, ALLOWED_ORIGINS (../.env dimuat bila ada)
go run ./cmd/server
```

Backend berjalan di port dari `PORT` (default 8080 di konfigurasi proyek ini).

Variabel penting:

| Variabel | Isi |
|---|---|
| `DATABASE_URL` | DSN PostgreSQL, mis. `postgres://user:pass@localhost:5432/qms_db?sslmode=disable` |
| `JWT_PUBLIC_KEY_FILE` | Kunci publik RSA untuk memverifikasi token login (RS256) dari auth-service. Wajib |
| `PORT` | Port HTTP backend |
| `ALLOWED_ORIGINS` | Origin frontend yang diizinkan (dipisah koma) |
| `PUBLIC_APP_URL` | Origin frontend untuk `join_url`, mis. `http://localhost:8080` (lewat nginx) |
| `QR_HTTP_TIMEOUT` | Timeout panggilan internal ke payment-service (detik, default 1) |
| `EMAIL_VERIFICATION_REQUIRED` | Guard verifikasi email di backend. Kosong = nyala; `false` mematikan (semua akun dianggap terverifikasi). Harus sama dengan nilai di auth-service (di sana kosong = nyala bila `MAIL_SERVICE_URL` terisi, dan `true` tanpa mail-service ditolak saat start) |
| `SHUTDOWN_TIMEOUT` | Detik menunggu request selesai saat backend berhenti (default 8; compose dev 2). Harus lebih kecil dari `stop_grace_period` |

Catatan: `PUBLIC_APP_URL` harus bisa dibuka dari HP yang memindai QR (jangan `localhost` untuk uji perangkat).

#### 5. Frontend

```bash
cd frontend
npm install
npm run web          # http://localhost:8081
npm run web:lan      # bisa diakses dari perangkat lain di jaringan yang sama
```

Alamat API diatur lewat `EXPO_PUBLIC_API_URL`. Kalau kosong, frontend memakai host yang dibuka dengan port 8080.

### Menjalankan tes

```bash
# Unit backend (tanpa database; paket integrasi ikut ter-skip tanpa build tag)
cd backend
go test ./... -count=1

# Integration backend (butuh DATABASE_URL yang bisa diakses; memakai schema sementara)
go test -tags integration ./internal/test/integration_test/ -count=1 -v

# auth-service, notify-service, mail-service, payment-service (masing-masing modul Go sendiri)
cd auth-service && go test ./...
cd notify-service && go test ./...
cd mail-service && go test ./...
cd payment-service && go test ./...

# Unit frontend
cd frontend
npm run test:unit
```

Tes tiap modul Go ada di `internal/test/<lapisan>_test/` (di notify-service juga `cmd/notify-service/main_test.go`; payment-service menaruh `*_test.go` di sebelah kodenya). Jumlah fungsi `Test*` per modul (dihitung dari kode): backend 516, auth-service 158, mail-service 83, notify-service 59, payment-service 22. Unit test frontend: 208. Angka cakupan statement (`go test ./... -coverpkg=./...`) terakhir diukur sebelum refactor terbaru (backend sekitar 70%, auth-service sekitar 86%, notify-service sekitar 84%, mail-service sekitar 76%); ukur ulang bila dibutuhkan.

End-to-end memakai Playwright di `frontend/playwright/` (`npm run e2e`): sekitar 254 pemanggilan `test(` dalam 33 file spec. Tes ini butuh backend dan frontend yang berjalan.

### Docker Compose

```bash
cp .env.example .env     # isi secret (QMS_PUBLIC_ORIGIN = origin yang dibuka dari HP)
docker compose up --build
```

Buka `http://localhost:8080` (atau `QMS_PUBLIC_ORIGIN`). Hanya nginx yang membuka port ke host;
Layanan internal (auth, notify, mail) tidak punya `ports:` sehingga endpoint internal-nya hanya bisa diakses dari jaringan Docker.

| Layanan | Dockerfile | Catatan |
|---|---|---|
| db | `db/Dockerfile` | `postgres:16-alpine`, skema dan index dijalankan otomatis saat data directory kosong |
| auth-service | `auth-service/Dockerfile` | Target `dev`, hanya di jaringan internal (:8094). Login, registrasi, login Google, reset password, verifikasi email. Satu-satunya pemegang kunci privat JWT; role database `auth_rw` |
| notify-service | `notify-service/Dockerfile` | Target `dev`, hanya di jaringan internal. LISTEN ke Postgres (role `notify_ro`), kirim Web Push (VAPID) ke browser dan FCM ke aplikasi Android (opsional, kredensial di `keys/fcm/`). Tidak bergantung pada email |
| mail-service | `mail-service/Dockerfile` | Target `dev`, hanya di jaringan internal (:8092). Satu-satunya pengirim email: API `POST /internal/v1/send` (`X-Service-Token`), templat, SMTP, coba ulang |
| payment-service | `payment-service/Dockerfile` | Target `dev`, hanya di jaringan internal. Pembayaran langganan QRIS lewat BorderPay; webhook publik masuk lewat nginx di `/api/v1/webhooks/borderpay` |
| mailpit | `axllent/mailpit:v1.31.6` | Hanya dev. Server SMTP palsu: menangkap semua email dari mail-service. SMTP dan UI hanya diikat ke 127.0.0.1 (`:1025` dan `:8025`), tidak lewat nginx; mail-service memakai `mailpit:1025` lewat jaringan internal |
| backend | `backend/Dockerfile` | Target `dev` (`go run`, restart manual), healthcheck `/health`, hanya di jaringan internal |
| frontend | `frontend/Dockerfile` | Target `dev` (Metro, hot reload) di port 80 internal |
| nginx | `deploy/nginx/Dockerfile` | Reverse proxy, satu-satunya yang membuka port (80) |

Satu `docker-compose.yml` untuk development; semua kode di-mount. Frontend hot reload; Go perlu `docker compose restart backend` (atau `auth-service`, `notify-service`, `mail-service`) setelah ubah kode. Untuk demo/produksi pakai `docker-compose.prod.yml` (lihat bawah).
Kredensial (`POSTGRES_PASSWORD` dan kata sandi role database) tidak dibakar ke image; kunci privat JWT di-mount dari `keys-dev/` (dev) atau `keys/` (prod).
Frontend memanggil API di origin halaman (`EXPO_PUBLIC_API_URL=origin`), jadi mengganti `QMS_PUBLIC_ORIGIN` tidak butuh build ulang frontend.

Untuk uji dari HP, jalankan `make up` (build + stack, alamat memakai IP LAN) atau `make tunnel` (HTTPS publik, kamera HP jalan); `make down` untuk berhenti. Perintah lain: `make restart` (backend, auth-service, notify-service, mail-service), `make seed`, `make logs`. Lihat `Makefile` dan `deploy/nginx/README.md`.

### Demo/produksi di belakang Cloudflare Tunnel

```bash
cp .env.prod.example .env.prod && chmod 600 .env.prod   # isi rahasia (openssl rand -hex 32), kunci VAPID baru, dan (opsional) FCM_*; .env dev tidak tersentuh
make down                   # stack dev memakai port 8080 yang sama
make prod-up                # cek konfigurasi (MAIL, Google, pembayaran), siapkan role DB, lalu docker compose up -d --build
make prod-payment-check     # hanya membaca: konfigurasi BorderPay, hak role payment_rw, status payment-service
make prod-seed-penguji       # akun penguji + pelanggan seed, sandi acak, ditulis ke ~/qms-penguji-account.txt
make prod-guard             # seed/reset dev ditolak di DB ini
make prod-backup            # ~/qms-backups/qms-<waktu>.sql.gz; restore: scripts/backup-prod.sh restore <file>
make prod-reset             # RESET TOTAL: hapus semua akun dan data prod, mulai kosong; backup otomatis dulu
```

Tunnel `qms-ta` adalah service systemd yang **manual**: `sudo systemctl start cloudflared` sebelum demo, `sudo systemctl stop cloudflared` sesudahnya (tidak hidup saat boot, berhenti otomatis saat laptop mati). Akses dibatasi aturan WAF Cloudflare berdasarkan IP WiFi. Webhook BorderPay wajib dikecualikan dari aturan itu; lihat [Langganan owner dan pembayaran BorderPay](#langganan-owner-dan-pembayaran-borderpay). Berkas `docs/setup-perintah-manual.md` dan `docs/petunjuk-mentor.md` (perintah tanpa `make` dan panduan mentor) tidak ada di repo ini.

Nginx hanya terikat ke `127.0.0.1:8080`; `cloudflared` di host menunjuk ke `http://localhost:8080`. IP klien dibaca dari `CF-Connecting-IP`. Dengan `APP_ENV=production`, backend menolak start bila `ALLOWED_ORIGINS` berisi `*` atau origin bukan https, atau `PUBLIC_APP_URL` bukan https; auth-service menolak start bila `PUBLIC_APP_URL` bukan https atau `MAIL_SERVICE_TOKEN` kurang dari 32 karakter. Pemeriksaan rahasia lemah (nilai contoh atau terlalu pendek) di backend saat ini dinonaktifkan (dikomentari di `backend/internal/config/config.go`); kekuatan rahasia dijaga oleh `make prod-up` (`scripts/check-*.sh`). Database prod memakai volume sendiri tanpa seed. Backend berjalan satu instance (hub SSE di memori). Catatan domain, WAF, dan rencana live (`docs/setup-domain.md`, `docs/setup-waf-wifi.md`, `docs/plan-live.md`) tidak ada di repo ini.

### Daftar script

Semua script berada di `scripts/` (dijalankan dari akar `queue-management-system/`) dan `db/scripts/`. Sebagian besar dipanggil lewat `make`; yang lain dijalankan langsung. Tabel ini menjelaskan kapan masing-masing dipakai; penjelasan rinci per skrip ada di [scripts/README.md](scripts/README.md).

| Script | Fungsi | Cara pakai | Dipanggil dari |
|---|---|---|---|
| `scripts/backup-prod.sh` | Backup (`pg_dump`, gzip) dan restore database prod. Restore **menimpa** data prod. | `make prod-backup`, atau `QMS_ENV_FILE=.env.prod scripts/backup-prod.sh backup`. Restore: `scripts/backup-prod.sh restore <file.sql.gz>` | `make prod-backup`, `make prod-reset` |
| `scripts/reset-demo.sh` | RESET TOTAL database prod: volume DB dihapus, stack dinyalakan lagi dengan DB kosong. Semua akun ikut hilang. Backup otomatis dibuat dulu. | `make prod-reset`, atau `scripts/reset-demo.sh [-y]` |
| `scripts/verify-user.sh` | Menandai satu email sebagai terverifikasi langsung di database. Dipakai bila email verifikasi tidak sampai. | `make verify-user EMAIL=<email> [STACK=prod]`, atau `scripts/verify-user.sh <email> [prod]` | `make verify-user` |
| `scripts/check-mail-mode.sh` | Memeriksa `MAIL_MODE` dan `SMTP_*` di `.env.prod` sebelum stack prod naik. Menolak konfigurasi yang salah. | Otomatis lewat `make prod-up`, atau `MAIL_MODE=smtp scripts/check-mail-mode.sh .env.prod` | `make prod-up` |
| `scripts/gen-auth-keys.sh` | Membuat pasangan kunci RSA untuk token login (`keys/`, tidak ikut git). Idempoten; tidak menimpa kunci yang ada. | Otomatis lewat `make up` dan `make prod-up` | `make up`, `make prod-up` |
| `scripts/check-google-config.sh` | Memeriksa konfigurasi login Google: `GOOGLE_CLIENT_IDS` (server) dan `EXPO_PUBLIC_GOOGLE_*` (frontend) harus sinkron. Berhenti bila tidak (login pasti gagal). | Otomatis lewat `make up`, `make tunnel`, `make prod-up` | `make up`, `make tunnel`, `make prod-up` |
| `scripts/check-service-isolation.sh` | Memastikan `notify-service` tidak bergantung pada mail-service, Mailpit, atau SMTP (dev dan prod), `auth-service` terpisah dari jalur antrian, kunci privat JWT hanya di-mount ke `auth-service`, dan overlay observability tidak mengganggu aplikasi (tanpa `depends_on` dari aplikasi, hanya Grafana di 127.0.0.1 yang membuka port, tanpa `docker.sock`, mount host read-only, semua komponen ber-`mem_limit`, compose dasar bersih dari OTEL_*, nginx tidak meneruskan `/metrics`). Membutuhkan `docker` dan `python3`. | `scripts/check-service-isolation.sh` (jalankan setelah mengubah `docker-compose*.yml` atau konfigurasi nginx) | `make obs-check` (bersama tes kebocoran) |
| `scripts/check-observability-leaks.sh` | Tes kebocoran rahasia: mengirim request ditolak berisi rahasia palsu (token layar, password, email, JWT, token verifikasi/reset), lalu memastikan tidak ada di Loki, Tempo, maupun label Prometheus. Butuh stack jalan dengan overlay, `curl`, `openssl`, `python3`. Kode keluar 0 bersih, 1 bocor, 2 stack observability tidak terjangkau. | `make obs-check` atau `scripts/check-observability-leaks.sh` | `make obs-check` |
| `db/scripts/apply_schema.sql` | Penyesuaian skema yang idempoten untuk database yang sudah berjalan. Tidak menyentuh data. | Dev: `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql`. Prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod`. Lalu restart backend, notify-service, dan mail-service. | Manual (lihat [Notifikasi](#notifikasi-notify-service) dan [Email](#email-mailpit-atau-smtp-nyata)) |
| `db/scripts/reset_seed_dev.sql` | Mengembalikan database dev ke data seed. Ditolak di prod setelah `make prod-guard`. | `make seed` | `make seed`, tes Playwright |
| `db/scripts/backfill_currently_served.sql` | Backfill sekali jalan: tiket `completed` terakhir di antrian aktif menjadi `currently_served`. Berakhir dengan `ROLLBACK`: periksa jumlah baris, lalu ganti ke `COMMIT`. Jalankan setelah backup dan `apply_schema.sql`. | Dev: `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/backfill_currently_served.sql` | Manual |
| `db/migrations/*.sh` (`03`, `04`, `06`, `07`) | Membuat role database `notify_ro`, `mail_rw`, `auth_rw`, dan `metrics_ro` (`07`, dilewati tanpa `METRICS_DB_PASSWORD`). | **Tidak dijalankan manual.** Docker menjalankannya saat volume database masih kosong (lihat `db/Dockerfile`). | Docker `initdb` |
| `db/migrations/08_payment_role.sh` | Membuat atau memperbarui role `payment_rw` (payment-service) dan grant-nya. Idempoten. | **Dijalankan ulang otomatis oleh `make prod-up`**, karena database prod yang sudah ada tidak menerima initdb. Hak-haknya diperiksa `db/scripts/check_payment_grants.sql`. | `make prod-up`, `make prod-payment-check` |

**Script yang sudah dihapus**

- `frontend/scripts/reset-project.js` (dan script npm `reset-project`) dihapus. Itu sisa template Expo yang memindahkan `src/` dan `scripts/` ke `example/`, dan tidak dipakai di proyek ini.

### Build frontend untuk produksi

```bash
cd frontend
EXPO_PUBLIC_API_URL=https://domain-anda npx expo export --platform web --clear
```

Hasilnya ada di `frontend/dist/`. Salin ke folder yang dibaca nginx, lalu ikuti [deploy/nginx/README.md](deploy/nginx/README.md).

### Konfigurasi layanan pendukung

Detail konfigurasi dan operasi untuk notify-service dan mail-service.

#### Notifikasi (notify-service)

Variabel `.env` (lihat `.env.example`): `NOTIFY_DB_PASSWORD` (password role `notify_ro`, hanya baca), `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`. Buat kunci VAPID dengan `npx web-push generate-vapid-keys`. Opsional untuk aplikasi Android: `FCM_PROJECT_ID` dan `FCM_CREDENTIALS_FILE` (lihat di bawah).

Alur: trigger `tickets_notify_trg` di Postgres memancarkan `pg_notify('queue_events')` saat nomor dipanggil atau dipanggil ulang; `notify-service` mendengarkan, menghitung penerima, lalu mengirim push ke setiap perangkat penerima: Web Push untuk browser (`push_subscriptions`) dan FCM untuk aplikasi Android (`push_devices`). Tanpa outbox, jadi push yang terjadi saat service mati hilang.

Push ke aplikasi Android (FCM), langkah sekali pasang:
1. Firebase Console: buat project (atau pakai yang ada), tambah aplikasi Android dengan package `my.id.montechristo.qms`, dan pastikan **Firebase Cloud Messaging API (V1)** aktif (Project settings > Cloud Messaging). Langkah "Add Firebase SDK" (Gradle) dilewati: Expo memasang plugin Gradle sendiri saat build.
2. `google-services.json` (Project settings > General > Your apps): simpan di `frontend/google-services.json` (di-gitignore) untuk build lokal, dan daftarkan ke EAS karena EAS Build tidak mengunggah berkas yang di-gitignore: `cd frontend && npx eas-cli env:create --name GOOGLE_SERVICES_JSON --type file --value ./google-services.json --visibility secret --environment production`. `frontend/app.config.js` membacanya.
3. Kredensial server (Project settings > Service accounts > Generate new private key): simpan sebagai `keys/fcm/fcm-sa.json`, lalu beri notify-service (UID 10001) akses baca lewat ACL tanpa mengganti pemilik: `chmod 700 keys/fcm && chmod 600 keys/fcm/fcm-sa.json && setfacl -m u:10001:rx keys/fcm && setfacl -m u:10001:r keys/fcm/fcm-sa.json` (pola yang sama dengan `scripts/gen-auth-keys.sh`). Jangan `chown` ke 10001: folder jadi tidak terbaca oleh user Anda dan EAS Build gagal dengan `EACCES: permission denied, scandir 'keys/fcm'`, karena EAS memindai seluruh repo. Pakai `keys/fcm/` juga di dev, bukan `keys-dev/` yang dihapus setiap `make up`. Bila `keys/fcm` sudah ada milik root (dibuat Docker saat mount), ambil alih dulu: `sudo chown -R "$(id -un)": keys/fcm`.
4. Isi `.env` / `.env.prod`: `FCM_PROJECT_ID=<project id>` dan `FCM_CREDENTIALS_FILE=/run/fcm/fcm-sa.json` (`keys/fcm` di-mount ke `/run/fcm`). Jalankan `apply_schema.sql` (tabel `push_devices`), lalu restart backend dan notify-service.
5. Cek: log notify-service menampilkan `FCM push enabled`, dan `POST /api/v1/push/fcm/register` tanpa login menjawab 401 (bukan 404).
6. Build APK baru (`npx eas-cli build -p android --profile production`); APK lama tidak punya kode notifikasi dan tidak bisa diperbaiki lewat EAS Update. Di aplikasi: halaman tiket > AKTIFKAN NOTIFIKASI > izinkan. Token tersimpan di `push_devices`.

Tanpa `FCM_*`, Web Push tetap berjalan; log hanya mencatat peringatan bila konfigurasinya setengah atau berkasnya tidak terbaca. Metrik `push_sent_total` dan `push_failed_total` punya label `channel` (`webpush` atau `fcm`).

Catatan:
- Satu browser atau satu HP hanya melayani satu akun. Layar tiket mendaftarkan ulang langganan (atau token FCM) ke akun yang sedang login, dan logout melepasnya.
- Push hanya jalan di HTTPS (domain tunnel `qms.montechristo.my.id`, atau `make tunnel` untuk uji sementara) dan di browser yang mendukung. iPhone/iPad perlu situs dipasang ke Layar Utama. Aplikasi Android memakai FCM (development/production build); Expo Go dan iOS native tidak didukung, tombol notifikasi tidak tampil di sana.
- DB yang sudah berjalan tidak menjalankan ulang skrip init. **Cara cepat dan idempoten:** `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql` (prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod`), lalu restart backend dan notify-service. Isinya mencakup semua penyesuaian di bawah ini, yang bila dikerjakan manual: `ALTER TABLE tickets ADD COLUMN recalled_at TIMESTAMPTZ;`, tabel `push_subscriptions` dan `push_devices`, fungsi `tickets_notify` dan trigger `tickets_notify_trg` (jika sudah ada versi lama tanpa event `skipped` atau `requeued`, jalankan ulang `CREATE OR REPLACE FUNCTION` dan buat ulang trigger) (ada di `db/migrations/01g_triggers.sql`), serta role `notify_ro` (`db/migrations/03_notify_role.sh`). Pengaturan Skip juga butuh kolom `queue_settings.skip_fallback`, `skip_fallback_n`, `skip_max_returns`.
- Awalan nomor tiket: DB lama butuh `ALTER TABLE queues ADD COLUMN ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A' CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$');`, lalu restart backend dan notify-service (notify membaca kolom ini).
- Tes unit: `cd notify-service && go test ./...`.

#### Langganan owner dan pembayaran BorderPay

Owner membayar **1 bulan langganan (30 hari)** lewat QRIS BorderPay. Harga tetap Rp15.000 per bulan: `SubscriptionPriceIDR` di `backend/internal/config/config.go` diisi konstanta 15000 dan **tidak dibaca dari env** (`SUBSCRIPTION_PRICE_IDR` di compose dan `.env.prod.example` tidak berefek). Alur:

1. Frontend memanggil `POST /api/v1/owners/subscription/checkout` (butuh JWT dan email terverifikasi, metode `qris`). Backend menentukan harga dan membuat `reference_id` (`SUB-<owner>-<unix>-<acak>`).
2. Backend memanggil payment-service di `/internal/v1/payments` dengan `X-Service-Token`. payment-service mencatat pesanan `pending` di `payment_orders`, lalu membuat transaksi di BorderPay dan menyimpan `pay_url`.
3. Frontend membuka `pay_url`. Customer membayar di halaman BorderPay.
4. BorderPay mengirim `payment.paid` ke `https://<domain>/api/v1/webhooks/borderpay` dengan header `x-borderpay-token`. payment-service memverifikasi token, lalu dalam **satu transaksi**: pesanan menjadi `paid`, `owners.subscription_expires_at = GREATEST(now(), berakhir) + 30 hari` (baris `owners` dibuat untuk calon owner pertama), dan satu baris `payments` dicatat. Webhook kembar tidak menerapkan pembayaran dua kali (idempoten).

Backend hanya *membaca* `owners.subscription_expires_at` (402 `SUBSCRIPTION_INACTIVE` bila lewat). Pembayaran tidak mengubah antrian. Kode: `payment-service/internal/{checkout,webhook,store,borderpay}`, `backend/internal/service/subscription_checkout.go`, `backend/internal/paymentclient`.

**Arti respons webhook** (lihat juga log Dashboard BorderPay):

| Status | Arti | Tindakan |
|---|---|---|
| `200` | Diterapkan, atau sudah pernah diterapkan, atau `reference_id` tidak dikenal (diabaikan) | Tidak perlu tindakan |
| `401` | `x-borderpay-token` tidak cocok dengan `BORDERPAY_WEBHOOK_TOKEN` | Samakan token dengan dashboard BorderPay |
| `403` (di BorderPay, tidak ada di log nginx) | WAF Cloudflare memblokir request sebelum sampai ke server | Lihat bagian Cloudflare di bawah |
| `409` | Nominal webhook tidak sama dengan pesanan | Pembayaran tidak diterapkan; periksa manual |
| `500` | Penyimpanan ke database gagal (lihat log `simpan pembayaran gagal`) | BorderPay mengirim ulang; perbaiki penyebab di log dulu |

**Persiapan prod** (setiap kali rahasia pembayaran diganti atau server baru):

- Isi `BORDERPAY_API_KEY` (`bp_live_...`), `BORDERPAY_WEBHOOK_TOKEN`, `PAYMENT_SERVICE_TOKEN`, dan `PAYMENT_DB_PASSWORD` di `.env.prod`. Daftarnya ada di `.env.prod.example`.
- Di dashboard BorderPay mode **Live**, isi endpoint `https://<QMS_PUBLIC_ORIGIN>/api/v1/webhooks/borderpay`, aktifkan event `payment.paid`, dan samakan verification token dengan `BORDERPAY_WEBHOOK_TOKEN`. Mode Test punya endpoint terpisah.
- Jalankan `make prod-payment-check`. Semua pemeriksaan harus lulus (peringatan kunci test muncul bila masih `bp_test_`).
- Cloudflare: rule WAF yang membatasi akses per IP WiFi harus mengecualikan path webhook. Ekspresi yang dipakai:
  `(http.host eq "<domain>" and not ip.src in {<IP WiFi>} and not (http.request.uri.path eq "/api/v1/webhooks/borderpay"))`.
  Tanpa pengecualian ini, BorderPay menerima 403 dan pembayaran tidak tercatat, padahal pembayarannya sudah lunas.

**Yang dijaga otomatis oleh `make prod-up`** (sehingga kegagalan di atas tidak terulang diam-diam):

- `scripts/check-payment-config.sh` menolak start bila rahasia kosong, pendek, atau masih contoh (`GANTI`). Nilai rahasia tidak pernah dicetak.
- Profil `payments` selalu aktif di prod dan `payment-service` dibangun ulang dari kode sekarang, sehingga image lama tidak tertinggal.
- Database `db` dinyalakan dulu, lalu `db/migrations/08_payment_role.sh` dijalankan ulang (idempoten). Skrip ini hanya jalan saat volume kosong, jadi tanpa langkah ini grant yang ditambahkan kemudian tidak pernah sampai ke database prod.
- `db/scripts/check_payment_grants.sql` memastikan `payment_rw` punya semua hak yang dibutuhkan (termasuk `INSERT` pada `owners`, yang diperlukan `ON CONFLICT`). Bila ada yang kurang, `make prod-up` berhenti dan menyebut hak yang hilang.

**Pemecahan masalah:**

| Gejala | Penyebab paling mungkin | Cek |
|---|---|---|
| Checkout `503 PAYMENT_UNAVAILABLE` | payment-service mati atau restart, atau token backend dan service tidak sama | `make prod-payment-check`, lalu `docker compose -f docker-compose.prod.yml --env-file .env.prod logs --tail=100 payment-service` |
| payment-service restart terus | `variabel wajib kosong` (config) atau `koneksi database gagal` (password atau role) | Log yang sama dengan perintah di atas |
| Webhook `500`, log `permission denied for table owners` | Hak `payment_rw` kurang | `make prod-payment-check`, lalu `make prod-up` |
| Pembayaran sudah lunas di BorderPay, langganan tidak bertambah, pesanan `pending` | Webhook tidak sampai (401, 403, atau 500) | Log pengiriman di dashboard BorderPay; `docker logs -f qms-prod-nginx-1 2>&1 \| grep borderpay` |
| Webhook `403` di BorderPay, tidak ada baris di log nginx | WAF Cloudflare memblokir | Cloudflare → Security → Events, filter path webhook |

Setelah penyebabnya diperbaiki, kirim ulang dari dashboard BorderPay. Webhook yang sama aman dikirim ulang karena idempoten. Jangan menambahkan langganan secara manual ke database tanpa bukti pembayaran dari BorderPay.


#### Email (Mailpit atau SMTP nyata)

`mail-service` adalah satu-satunya pengirim email (`notify-service` hanya push, Web Push dan FCM, dan tetap jalan walau email mati; dijaga `scripts/check-service-isolation.sh`). Pemanggil (auth-service untuk reset password, verifikasi, dan konfirmasi terverifikasi) memakai `POST /internal/v1/send` dengan header `X-Service-Token` (`MAIL_SERVICE_TOKEN`). Pengaturan SMTP lewat `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_TLS` (`none`, `starttls`, atau `tls`); divalidasi saat start dan service menolak start bila salah atau tidak lengkap.

##### Email notifikasi tiket (outbox)
Empat email ke pemilik tiket (masuk antrian, dibatalkan, nomor dihapus owner, selesai dilayani) dan satu email `queue-closed` ke owner saat ia menutup antriannya sendiri ditulis ke tabel `notification_outbox` oleh trigger (`tickets_email_outbox`, `queues_email_outbox`) dalam transaksi yang sama dengan perubahan tiket atau status antrian, lalu dikirim oleh worker di `mail-service` (role database `mail_rw`, `MAIL_DB_PASSWORD`). Email yang gagal dicoba ulang dengan jeda berlipat; bila mail-service atau SMTP mati, email menunggu di outbox dan terkirim setelah hidup lagi, sementara join, cancel, dan push tidak terpengaruh. Pengguna bisa mematikannya di Profil (`Notifikasi email`, kolom `users.email_notifications`).

Email `queue-closed`: saat owner menutup antrian (status `active` ke `closed`), trigger `queues_email_outbox` menulis satu baris outbox (`kind='queue-closed'`, `ticket_id` kosong, `queue_id` terisi) untuk owner, selama `email_notifications` miliknya menyala. Mail-service mengirimnya dengan tautan `<origin publik>/riwayat`. Di tabel outbox, `ticket_id` boleh NULL, ada kolom `queue_id`, index unik parsial `uq_outbox_kind_queue`, dan CHECK `kind` memuat `queue-closed`.

**DB yang sudah berjalan** (skrip init hanya jalan pada data directory kosong): terapkan skema (wajib untuk perubahan outbox di atas) dan buat role:
```
docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
docker compose exec -T -e MAIL_DB_PASSWORD="$(grep ^MAIL_DB_PASSWORD .env | cut -d= -f2)" db sh < db/migrations/04_mail_role.sh
docker compose up -d mail-service
```
(prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod` dan ambil `MAIL_DB_PASSWORD` dari `.env.prod`). Tes: `QMS_DOCKER_EXEC=1 npx playwright test ts-30` (dua tes ketahanan mematikan dan menyalakan container).

##### Verifikasi email
Akun baru dikirimi tautan `/verify-email?token=...` (berlaku 24 jam, sekali pakai, maksimal 3 kiriman per jam per akun; yang disimpan hanya hash tokennya di `user_action_tokens`). Pengiriman di latar belakang (auth-service lewat mail-service): bila gagal, registrasi tetap berhasil dan user bisa kirim ulang dari halaman Profil. Akun yang belum terverifikasi tidak bisa bergabung ke antrian (nomor baru) atau membuat antrian (403 `EMAIL_NOT_VERIFIED`); status dibaca dari database, jadi berlaku tanpa login ulang. Menyala bila `EMAIL_VERIFICATION_REQUIRED` kosong atau `true` (di auth-service bawaan ikut adanya mail-service).

Begitu token verifikasi dipakai untuk akun yang sebelumnya belum terverifikasi, auth-service mengirim email `email-verified` (konfirmasi) lewat mail-service. Pengiriman ini best effort dan asinkron: tidak pernah mengubah hasil HTTP verifikasi, dan tidak ada email bila mail-service tidak dikonfigurasi.

| Lingkungan | Email verifikasi | Cara memverifikasi |
|---|---|---|
| Dev (`make up`) | Ditangkap Mailpit | Buka `http://localhost:8025`, klik tautan di email |
| Prod `MAIL_MODE=smtp` | Dikirim ke inbox asli lewat SMTP, mis. Gmail (`smtp.gmail.com`, 587, `starttls`, kata sandi aplikasi) | Pengguna klik tautan di inbox-nya |
| Prod `MAIL_MODE=mailpit` (**bawaan**) | Ditangkap Mailpit, **tidak terkirim ke siapa pun** | Ambil tautan dari Mailpit (SSH tunnel), atau `make verify-user EMAIL=<email> STACK=prod` |

- `make verify-user EMAIL=<email> [STACK=prod]` (`scripts/verify-user.sh`) menandai satu akun terverifikasi langsung di database. Idempoten dan aman terhadap karakter aneh di email.
- `scripts/check-mail-mode.sh` (dijalankan `make prod-up`) memperingatkan bahwa pengguna baru tidak bisa memverifikasi di mode `mailpit`, kecuali `EMAIL_VERIFICATION_REQUIRED=false`.
- **DB yang sudah berjalan:** `db/scripts/apply_schema.sql` menambah kolom `users.email_verified_at`, memperluas CHECK `purpose`, dan **sekali saja** menandai semua akun yang sudah ada terverifikasi (agar tidak terkunci); menjalankannya ulang tidak memverifikasi akun baru. Lalu restart backend, auth-service, dan mail-service (templat `email-verify` dan `email-verified` ada di image mail-service). Seed dev dan akun demo dibuat sudah terverifikasi.
- Tes: `ts-33-email-verification.spec.ts` (butuh Mailpit dan owner berlangganan). Helper `registerAccount` di tes e2e memverifikasi akun lewat Mailpit dan menghapus email verifikasinya; beri `{ verify: false }` untuk akun yang belum terverifikasi.

##### Link Mailpit
Mailpit hanya diikat ke `127.0.0.1` (UI `:8025`, SMTP `:1025`) dan **tidak lewat nginx**, jadi tidak terbuka lewat IP LAN maupun tunnel Cloudflare. Perangkat lain memakai SSH tunnel: `ssh -L 8025:127.0.0.1:8025 <user>@<IP mesin>`, lalu `http://localhost:8025`. Port diubah lewat `MAILPIT_UI_PORT` dan `MAILPIT_SMTP_PORT`. `make mailpit` menampilkan semua link tanpa menjalankan apa pun.

| Mode | Perintah | Email pergi ke | Link Mailpit |
|---|---|---|---|
| Dev (LAN) | `make up` | Mailpit (tidak keluar ke internet) | `http://localhost:8025` (dicetak di akhir `make up`) |
| Dev + tunnel | `make tunnel` | Mailpit | sama; tidak ikut lewat tunnel. Tautan di email memakai URL tunnel sesi itu |
| Prod, SMTP nyata | `make prod-up-smtp` (atau `MAIL_MODE=smtp` di `.env.prod`) | Penerima sebenarnya | Tidak ada Mailpit; lihat inbox penerima atau dashboard penyedia SMTP |
| Prod, Mailpit | `make prod-up` (bawaan), atau `make prod-up-mailpit` | Mailpit, **tidak terkirim ke siapa pun** | `http://localhost:8025` di server prod; dari jauh pakai SSH tunnel |

##### Memilih mode email produksi
`MAIL_MODE` di `.env.prod` (`mailpit` atau `smtp`; kosong = `mailpit`), atau satu kali jalan dari baris perintah: `make prod-up MAIL_MODE=mailpit` / `make prod-up MAIL_MODE=smtp` (menimpa file). Mode `mailpit` menyalakan container Mailpit lewat compose profile `mailpit` dan memaksa `SMTP_*` ke Mailpit (menimpa isi `.env.prod`, jadi kredensial SMTP nyata tidak dipakai). Pesan Mailpit prod disimpan di volume `mailpit_data_prod` (bertahan saat restart). Pindah ke `smtp` menghentikan container Mailpit; `make prod-down` menghentikannya di kedua mode. `scripts/check-mail-mode.sh` dijalankan oleh `make prod-up` dan menolak: mode selain `smtp`/`mailpit`, `SMTP_*` kosong atau masih berisi `GANTI`, `SMTP_HOST=mailpit` pada mode `smtp`, `SMTP_TLS=none` ke host jauh, dan `MAIL_SERVICE_TOKEN` atau `MAIL_DB_PASSWORD` kosong, contoh, atau (token) kurang dari 32 karakter. Menjalankan `docker compose -f docker-compose.prod.yml` langsung (tanpa make) pada mode `mailpit` butuh `--profile mailpit` dan `SMTP_HOST=mailpit SMTP_PORT=1025 SMTP_TLS=none` di lingkungan.

##### Dev dan uji
- Compose dev sudah mengarahkan ke `mailpit:1025`. Pesan hilang saat container dibuat ulang. Uji jalur email tanpa menunggu peristiwa antrian: `docker compose exec mail-service go run ./cmd/mail-service send-test alamat@contoh.com`.
- **Produksi SMTP nyata:** `docker-compose.prod.yml` **mewajibkan** `MAIL_SERVICE_TOKEN`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` (lihat `.env.prod.example`). Gunakan `starttls` (587) atau `tls` (465) dengan kredensial dari penyedia dan alamat pengirim dari domain yang diverifikasi (SPF/DKIM).
- **Tes Playwright:** helper `playwright/unit-test/support/mailpit.ts` (`waitForMail`, `mailsTo`, `expectNoMail`, `uniqueRecipient`) membaca API Mailpit (`QMS_MAILPIT_URL`, bawaan `http://localhost:8025`); tes email di-skip bila Mailpit tidak menjawab. Pakai penerima unik per tes karena kotak surat dibagi semua tes.
- Memakai image baru? Pin versi di `docker-compose.yml` dan `docker-compose.prod.yml`; pull dari jaringan yang memeriksa sertifikat (proxy) bisa gagal, ulangi dari jaringan lain.

#### Log dan request ID
Service Go backend, auth, mail, dan notify menulis **satu baris JSON per kejadian** ke stdout lewat `slog`, dengan atribut `service`. payment-service belum: ia memakai paket `log` standar (teks), tanpa log akses JSON, metrik, maupun trace. Level diatur `LOG_LEVEL` (`debug`, `info`, `warn`, `error`; bawaan `info`). nginx menulis log akses JSON sendiri (lihat `deploy/nginx/README.md`).

- **Log akses** (backend, auth, mail): `request_id`, `method`, `route`, `status`, `latency_ms`, `bytes`, `client_ip`, dan `user_id` (angka saja, backend dan auth). `route` adalah **templat** (`/api/v1/display/:token`), bukan path mentah; rute yang tidak ada dicatat `unmatched`. Probe `/health` dan `/ready` dicatat di level `debug`. Panic dicatat sebagai `panic recovered` (dengan stack) dan klien menerima 500 `INTERNAL_ERROR`.
- **Request ID:** nginx membuat `X-Request-Id` untuk tiap request (nilai dari klien ditimpa) dan meneruskannya ke backend dan auth-service. ID dari luar yang kosong, lebih dari 128 karakter, atau berisi karakter selain huruf, angka, `-`, `_`, `.` diganti ID baru. Cari satu request di semua service dengan `docker compose logs | grep <request_id>`.
- **Kebijakan isi log.** Tidak boleh masuk log (termasuk log nginx dan, nanti, atribut span): JWT dan header `Authorization`, password, token layar publik, token reset password, token verifikasi email, kode join, token Web Push dan token FCM, isi body request, query string, dan alamat email penuh. Catat id angka (`user_id`, `ticket_id`, `queue_id`) atau templat route, bukan nilainya. Saat menambah log baru, jangan memasukkan error mentah dari pustaka yang bisa memuat URL atau kredensial tanpa memeriksanya.

#### Observability (opsional: log, metrik, trace, alert)
Overlay `docker-compose.observability.yml` menambah OpenTelemetry Collector, Loki, Prometheus, Tempo, dan Grafana (log, metrik, trace, dan alert; cakupannya backend, auth, mail, dan notify, bukan payment-service). Aplikasi tidak bergantung padanya: tidak ada `depends_on` ke arah sana, dan bila overlay mati antrian, login, SSE, dan langganan tetap jalan.

1. Isi `GRAFANA_ADMIN_PASSWORD` di `.env` (dev) atau `.env.prod` (prod). Wajib, tanpa nilai bawaan. `GRAFANA_PORT` dan `OBS_LOG_RETENTION` opsional.
2. `make obs-up` (dev) atau `make obs-up OBS_TARGET=prod`. Pertama kali, service aplikasi dibuat ulang sekali karena blok `logging:` berubah (volume dan database tetap). Di dev, nilai `QMS_PUBLIC_ORIGIN` backend yang sedang jalan dipakai ulang.
3. `make obs-url` menampilkan alamat Grafana: `http://localhost:3000`, **hanya di 127.0.0.1**. Dari perangkat lain pakai `ssh -L 3000:127.0.0.1:3000 <user>@<server>`. Login `admin` dan password tadi.
4. Dashboard **QMS: Log** (folder Grafana "Log", di-provision dari `observability/grafana/dashboards/Log/logs.json`; folder dibuat per jenis: Log, Metrik, Trace): jumlah log, ERROR, WARN, nginx 5xx, volume per service dan level, request nginx per kelas status, p95 latensi backend per route, route dengan 4xx/5xx, serta panel log dengan filter service, level, teks, dan `request_id`. Dashboard hanya diubah lewat berkas JSON di repo. Untuk query bebas: Grafana > Explore > Loki, contoh `{service="backend"} | json` atau `{service="nginx", level="ERROR"}`. Label Loki hanya `service` dan `level`; `request_id`, `route`, `status`, dan lainnya adalah structured metadata (`{service="backend"} | request_id="..."`). Satu `request_id` bisa dicari dari nginx sampai backend dan auth-service.
5. Verifikasi: `make obs-check` menjalankan pemeriksaan isolasi statis dan tes kebocoran rahasia (butuh stack jalan dengan `obs`). Jalankan setelah mengubah compose, nginx, atau kode logging/tracing.
6. Metrik (Prometheus): dashboard di folder "Metrik": **Ikhtisar (RED)** per service, **Antrian**, **Langganan dan email** (termasuk push), **Auth**, **Infrastruktur** (host, container, service Go), **PostgreSQL**, dan **nginx** (termasuk kelas status dan lonjakan 429 dari log). Tiap dashboard punya menu "Dashboard QMS" di kiri atas untuk pindah antar dashboard. Tiap service Go membuka `/metrics` di port **:9100 terpisah** (`METRICS_ADDR`, `off` mematikannya), tidak dipublikasikan ke host dan tidak melewati nginx; hanya Prometheus di jaringan `internal` yang mengambilnya. Bila port itu tidak bisa dipakai (mis. menjalankan beberapa service di satu host tanpa Docker), service tetap jalan dan hanya mencatat peringatan. Role database `metrics_ro` (hanya `pg_monitor`, tanpa akses ke data) dibuat otomatis oleh `make obs-up`; butuh `METRICS_DB_PASSWORD`. Retensi metrik 7 hari (`OBS_METRICS_RETENTION`), maks 1 GB.
7. Batasan: cAdvisor berjalan **tanpa `docker.sock`** (sesuai plan), jadi container dikenali dari id (12 karakter pertama sama dengan `docker ps`), bukan nama service. Memori per service Go ada di panel tersendiri lewat `process_resident_memory_bytes`. node_exporter memantau host (CPU, memori, disk), tetapi statistik jaringan yang terlihat milik container.
8. Trace (Tempo): dashboard **QMS: Trace** (folder "Trace") berisi peta service, laju/galat/latensi dari trace, daftar trace, dan log yang tertaut. Backend, auth, dan mail mengirim span ke collector (OTLP gRPC, jaringan internal) **hanya bila `OTEL_EXPORTER_OTLP_ENDPOINT` terisi**, yang diatur otomatis oleh overlay; tanpa overlay tracing no-op dan aplikasi start seperti biasa. Satu request join antrian menghasilkan satu trace: span request backend dan span query database. Sampling head: 10% di prod; di dev `make up obs`, `make tunnel obs`, dan `make obs-up` memakai 100% (semua trace) bila `OTEL_TRACES_SAMPLER_ARG` tidak diisi di `.env`, supaya dashboard Trace langsung terisi (nilai yang diisi eksplisit tetap dihormati); trace dari request yang error belum dipaksa 100% (tail sampling di luar cakupan). Retensi trace 3 hari (`OBS_TRACE_RETENTION`). Dari satu baris log, klik `trace_id` untuk membuka trace-nya; dari trace, tombol "Logs" membuka log service terkait. Pagar privasi: path mentah, query string, header, dan alamat klien dibuang dari span di aplikasi, nilai yang tersisa disamarkan (JWT, Bearer, token join, `token=`), dan collector menyamarkan sekali lagi; query database hanya templat (`$1`, `$2`), tanpa nilai parameter. Probe `/health` dan koneksi SSE tidak dilacak. nginx tidak menghasilkan span (image tanpa modul otel): trace dimulai di backend dan `X-Request-Id` dari nginx dicatat sebagai atribut span `request_id`.
9. Alert (Grafana, Alerting > Alert rules, folder "QMS Alert"): 11 aturan sebagai kode di `observability/grafana/provisioning/alerting/rules.yaml`: service mati, container start ulang berulang, rasio 5xx, latensi p95, antrean email menumpuk, login gagal melonjak, lonjakan langganan ditolak email belum diverifikasi, koneksi PostgreSQL, disk, memori host, dan lonjakan 429. Statusnya (Normal, Pending, Firing) terlihat di UI, tetapi **tidak ada notifikasi yang dikirim**: semua ditahan jadwal senyap `selalu-senyap` (`mutetimings.yaml`, `policies.yaml`). Ambang adalah tebakan awal dan disetel setelah ada data nyata. Mengirim notifikasi (email, Slack) menyusul: buat contact point di `contactpoints.yaml`, arahkan policy ke sana, lalu hapus `mute_time_intervals`. Perhatian: berkas alerting yang tidak valid membuat Grafana gagal start (modul provisioning gagal), jadi uji perubahan di sana dengan `docker compose ... up grafana` dan periksa lognya.
10. `make obs-down` menghapus container observability saja; log dan data Grafana tersimpan di volume `obs_*`. `make down` dan `make prod-down` ikut membersihkannya (`--remove-orphans`).

Rincian: log Docker (`json-file`, maks 3 x 10 MB per container) dibaca collector dari `/var/lib/docker/containers` (read-only, tanpa `docker.sock`), disamarkan lagi (Bearer, JWT, `token=`, `password=`) sebagai lapis kedua, lalu dikirim ke Loki (retensi 7 hari). Prometheus dan Tempo sudah berjalan di overlay yang sama (lihat butir 6 dan 8). Tambahkan kata `obs` agar observability ikut menyala: `make up obs`, `make tunnel obs`, `make prod-up obs` (butuh `GRAFANA_ADMIN_PASSWORD` dan `METRICS_DB_PASSWORD`). Menjalankan `make up` atau `make prod-up` tanpa kata itu membuat ulang service aplikasi tanpa blok `logging:`, sehingga log-nya berhenti terkumpul sampai `make obs-up` dijalankan lagi.

## 3. What was built

### Progres per komponen

| Komponen | Status | Catatan |
|---|---|---|
| Backend (Go, Gin) | Lengkap dan terhubung | Semua route punya handler, service, dan repository. Tanpa stub dan tanpa TODO. 516 fungsi tes (`go test ./... -count=1`). Tes ada di `internal/test/<lapisan>_test/`. |
| Frontend (Expo) | Lengkap dan terhubung | Semua layar memakai API nyata. 208 unit test dan sekitar 254 tes Playwright (33 spec). Login Google tersambung ke auth-service. |
| auth-service | Lengkap | Login, registrasi, login Google, reset password, verifikasi email, token RS256. 158 fungsi tes. |
| notify-service | Lengkap | LISTEN Postgres, Web Push, FCM untuk aplikasi Android, coba ulang, dan `/health`. Tanpa outbox. 59 fungsi tes. |
| mail-service | Lengkap | Outbox dengan coba ulang dan backoff, sembilan templat (`joined`, `cancelled`, `removed`, `completed`, `queue-closed`, `email-verify`, `email-verified`, `password-reset`, `test`), SMTP. 83 fungsi tes. |
| payment-service | Lengkap (tanpa log JSON, metrik, dan trace) | Checkout langganan QRIS lewat BorderPay (`/internal/v1/payments`, `X-Service-Token`) dan webhook `payment.paid` idempoten. Tidak memverifikasi JWT: hanya backend yang melakukannya, payment-service dijangkau lewat token layanan. 22 fungsi tes. |
| Database | Lengkap | 16 tabel, trigger notifikasi dan outbox email, role terpisah per service. |
| Deployment | Berjalan | Compose dev dan prod, nginx dengan rate limit, tunnel Cloudflare manual, Makefile dan skrip bantu. |

### Fitur yang ada

| Fitur | Penjelasan singkat |
|---|---|
| Akun | Daftar, masuk dengan email atau username, keluar, profil, avatar, dan preferensi. Login Google (opsional, bila `GOOGLE_CLIENT_IDS` diisi). Lupa dan reset password lewat email bila mail-service dipasang. Kolom konfirmasi password menampilkan "Password tidak sama" inline setelah blur atau submit. Mengetuk foto profil membuka popup foto yang diperbesar dengan tombol X. |
| Verifikasi email | Tautan verifikasi saat registrasi, halaman publik `/verify-email`, banner dan tombol kirim ulang di Profil. Belum terverifikasi: tidak bisa bergabung ke antrian atau membuat antrian. Dev lewat Mailpit, prod lewat `MAIL_MODE`. |
| Antrian | Owner membuat antrian (butuh langganan aktif), mengubah nama dan deskripsi, dan menutup antrian. |
| Panggilan nomor | Owner memakai Next, Skip (dengan aturan nomor yang dilewati), dan Panggil ulang. Customer bisa membatalkan tiketnya, dan owner bisa menghapus tiket. |
| QR dan tautan join | QR statis atau dinamis dengan kode join 6 karakter dari backend, penggantian kode statis, dan salin tautan join. Customer hanya bisa join lewat kode join. |
| Customer | Bergabung lewat QR atau tautan, melihat nomor yang sedang dilayani dan posisi di depan, serta riwayat tiket. |
| Catatan | Owner menulis catatan untuk nomor yang sedang dilayani. |
| Counter | Owner menambah, mengganti nama, dan menghapus counter per antrian lewat UI (`CounterBar`). Layar publik menampilkan satu kartu per counter. |
| Layar publik | Owner mengaktifkan layar di `/display/<token>` untuk monitor ruang tunggu, tanpa login. Satu kartu per counter, langsung berubah saat Next atau Skip. "GANTI TAUTAN" mematikan tautan lama. |
| Notifikasi giliran | Web Push (browser) dan FCM (aplikasi Android) dari `notify-service`: "Giliran Anda" saat dipanggil, "Tinggal 1 nomor lagi" untuk nomor berikutnya, dan pemberitahuan untuk nomor yang dilewati. Di browser aktif otomatis saat join bila izin diberikan (HTTPS, `make tunnel`); di aplikasi Android lewat tombol AKTIFKAN NOTIFIKASI di halaman tiket, dan ketuk notifikasi membuka halaman tiket. |
| Email | `mail-service` mengirim email dari outbox (tiket, dan `queue-closed` ke owner saat antrian ditutup) dan mencoba ulang bila gagal; auth-service mengirim email verifikasi, `email-verified`, dan reset password lewat mail-service. Pengguna bisa mematikan email notifikasi di Profil. |
| Riwayat dan ekspor | Owner melihat riwayat antrian dan bisa mengekspornya ke CSV atau XLSX. Customer melihat riwayat tiketnya. |
| Langganan | Owner membayar 1 bulan (30 hari) lewat QRIS BorderPay. Langganan aktif setelah webhook `payment.paid` terverifikasi dan diterapkan. |
| Real-time | Pembaruan lewat Server-Sent Events, termasuk `queue_qr_updated`. |
| Navigasi keyboard (web) | Di halaman masuk dan daftar, panah atas/bawah memindahkan fokus antar field dan tombol; Enter menekan tombol yang sedang fokus (di field teks, Enter ditangani form). |
| Tampilan | Mode terang dan gelap. |
| Shutdown rapi | Backend menangani SIGINT/SIGTERM: koneksi SSE ditutup, request yang berjalan dituntaskan (paling lama `SHUTDOWN_TIMEOUT`), lalu database ditutup. `ReadHeaderTimeout` 5 dtk, `ReadTimeout` 30 dtk, `IdleTimeout` 120 dtk. |

Yang belum ada dan alasannya ada di [bagian 4](#4-what-was-deferred-and-why).

### Struktur folder

```
queue-management-system/
├── backend/                  API Go (Gin, sqlx): antrian, tiket, counter, riwayat, SSE, langganan dan checkout
│   ├── cmd/server/           titik masuk (main.go): membaca config lalu memanggil app.Wire dan app.NewEngine
│   └── internal/
│       ├── app/              pemasangan dependensi (Wire) dan engine Gin (NewEngine), dipanggil dari cmd/server
│       ├── apperror/         error bersama (sentinel) untuk repository, service, dan handler
│       ├── config/           membaca dan memvalidasi env var
│       ├── database/         koneksi PostgreSQL dan pembungkus transaksi (WithTx)
│       ├── dto/              bentuk request dan response JSON
│       ├── export/           ekspor riwayat antrian ke CSV dan XLSX
│       ├── handler/          endpoint HTTP
│       ├── httpserver/       pembuatan dan shutdown server HTTP
│       ├── imageproc/        validasi, crop, dan perkecil foto profil
│       ├── logging/          log JSON (slog)
│       ├── metrics/          metrik Prometheus
│       ├── middleware/       verifikasi JWT (kunci publik), CORS, pencabutan sesi
│       ├── model/            struct data
│       ├── paymentclient/    klien HTTP ke payment-service (checkout langganan)
│       ├── repository/       akses data ke PostgreSQL
│       ├── requestid/        pembuatan dan penerusan ID request
│       ├── router/           pendaftaran rute HTTP
│       ├── service/          logika bisnis dan aturan antrian
│       ├── sse/              hub Server-Sent Events (di memori, satu hub, room per antrian)
│       ├── telemetry/        tracing OpenTelemetry (no-op bila tanpa OTEL_EXPORTER_OTLP_ENDPOINT)
│       ├── validator/        aturan validasi kustom, mis. no_html_tags
│       └── test/             tes per lapisan: <lapisan>_test/ (termasuk integration_test)
├── auth-service/             login, registrasi, Google, reset password, verifikasi email, :8094
│   ├── cmd/auth-service/     titik masuk: hanya merangkai komponen
│   └── internal/             apperror, config, database, dto, handler, httpserver, middleware, model, repository, router, service, validator,
│                             google (verifikasi ID token Google), mailclient (klien HTTP ke mail-service), test/<lapisan>_test
├── frontend/                 aplikasi Expo: src/app (rute), components, hooks, lib, store, utils, constants, types
├── notify-service/           Web Push dan FCM (LISTEN Postgres), /health :8091
│   ├── cmd/notify-service/   titik masuk (+ main_test.go)
│   └── internal/             apperror, config, dto, httpserver, model, repository, service,
│                             listener (LISTEN pg_notify), pusher (kirim Web Push, coba ulang, Dispatcher per kanal),
│                             fcm (kirim FCM HTTP v1 ke Android), runner, test/<lapisan>_test (tanpa handler dan router)
├── mail-service/             pengirim email (outbox dan SMTP), :8092
│   ├── cmd/mail-service/     titik masuk
│   └── internal/             apperror, config, dto, handler, middleware, model, repository, router, service,
│                             mailer (SMTP), mailtemplate (templat), test/<lapisan>_test
├── payment-service/          checkout langganan QRIS BorderPay dan webhook, :8095
│   ├── cmd/payment-service/  titik masuk
│   └── internal/             borderpay (klien API), checkout, webhook, order, store, config; tes di sebelah kode
├── db/                       Dockerfile, migrations (skema, indeks, role per service), dan scripts (apply_schema, reset_seed_dev, backfill)
├── deploy/nginx/             konfigurasi nginx (reverse proxy, rate limit)
├── observability/            overlay opsional: otel-collector, loki, prometheus, tempo, grafana (dashboard dan alert sebagai kode)
├── scripts/                  backup-prod, reset-demo, verify-user, gen-auth-keys, check-mail-mode, check-google-config, check-service-isolation (lihat "Daftar script")
├── keys/ dan keys-dev/       pasangan kunci RSA token login (prod dan dev), dibuat gen-auth-keys.sh
├── docker-compose.yml        stack development (hot reload)
├── docker-compose.prod.yml   stack demo/produksi (nginx hanya di 127.0.0.1)
├── docker-compose.observability.yml   overlay observability (dipasang lewat kata `obs`)
├── Makefile                  perintah umum: make up, make tunnel, make prod-up, dan lainnya
├── .env.example              template env dev
└── .env.prod.example         template env produksi
```

DESIGN.md, FLOW.md, dan api-contract.md ada di akar folder ini. Dokumen operasional lain yang disebut di bagian 2 (`docs/setup-*.md`, `docs/plan-*.md`, `docs/petunjuk-mentor.md`) tidak ada di repo ini.

## 4. What was deferred and why

Kolom "Pilihan?" menandai apakah penundaan itu keputusan yang tercatat (Ya) atau hanya belum dikerjakan (Belum).

| Item | Kondisi sekarang | Pilihan? | Alasan atau catatan |
|---|---|---|---|
| Backend lebih dari satu instance | Hub SSE di memori | Ya | Butuh pub/sub bersama (LISTEN/NOTIFY atau Redis). Demo memakai satu instance (`DESIGN.md` §2.7). |
| Outbox untuk Web Push | Push yang terjadi saat `notify-service` mati hilang | Ya | Email sudah memakai outbox; push dibuat lebih sederhana dulu. Rencana di `plan-setelah-live.md` (tidak ada di repo ini). |
| Migration runner | `db/migrations/01*.sql` hanya jalan pada data directory kosong; DB lama memakai `db/scripts/apply_schema.sql` yang idempoten | Ya | Cukup selama skema jarang berubah. |
| HSTS dan Cloudflare Access | Belum aktif; TLS dipegang Cloudflare, nginx hanya HTTP | Ya | Menunggu HTTPS stabil dan Zero Trust siap. |
| Plan langganan lain (tahunan, sekali bayar) dan penagihan berulang | Hanya paket `monthly` 30 hari lewat QRIS BorderPay; tidak ada penagihan otomatis | Ya | Kolom `plan_type` dan CHECK di `payment_orders` serta `payments` sudah menerima `yearly` dan `one_time`, tetapi belum ada harga dan alur untuk keduanya. |
| Ekspor file dan push di aplikasi native | Hanya jalan di web | Ya | Web Push hanya di browser lewat HTTPS; Expo Go tidak mendukung. |
| `registration_limit`, batas harian QR, batas riwayat, penghapusan permanen | Kolom skema ada, kode tidak memakainya | Ya | Dihapus dari produk pada 31 Okt 2026. |
| Backup otomatis | Hanya manual lewat `make prod-backup`, tanpa jadwal, retensi, atau salinan di luar mesin | Ya | Tidak diperlukan; cukup untuk demo. |
| Tes render komponen frontend | Tidak ada; hanya unit test logika dan Playwright | Ya | Logika murni diuji unit dan layar diuji Playwright; render test butuh `jest-expo` plus mock Reanimated/router dengan nilai tambah kecil. |
| Cek MX atau allowlist domain email | Tidak ada; verifikasi hanya lewat tautan | Ya | Kepemilikan email terbukti dengan klik tautan; domain yang tidak ada gagal terkirim dan akunnya tetap belum terverifikasi. |

## 5. Architecture

### Teknologi

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27, Gin, sqlx + pgx, verifikasi JWT RS256 (kunci publik), slog, metrik Prometheus, tracing OpenTelemetry |
| auth-service | Go 1.27, Gin, sqlx + pgx, penerbit JWT RS256 (kunci privat), bcrypt, verifikasi ID token Google |
| notify-service | Go 1.27, sqlx + pgx, LISTEN Postgres (pgx langsung), Web Push (VAPID), FCM HTTP v1; `/health` memakai `net/http` |
| mail-service | Go 1.27, Gin, sqlx + pgx, LISTEN Postgres (pgx langsung), SMTP, templat HTML dan teks, outbox di Postgres |
| payment-service | Go 1.27, `net/http` (tanpa Gin), sqlx + pgx, klien BorderPay QRIS |
| Database | PostgreSQL 16 |
| Frontend | Expo SDK 57 (Expo Router, web static), React Native, NativeWind, Zustand |
| Deploy | Docker Compose, nginx (reverse proxy + rate limit login dan `/join`); observability opsional: OpenTelemetry Collector, Prometheus, Loki, Tempo, Grafana |

### Gambaran sistem

```
Browser (Expo web / native)
   │  HTTP (REST, JSON) + SSE
   ▼
nginx ── frontend statis (dist)
   ├─ /api/v1/users/(login|register|google|forgot-password|reset-password|verify-email|me/verify-email/resend)
   │                         ─► auth-service :8094 ─► PostgreSQL (role auth_rw)
   │                                 └─► mail-service :8092 (email verifikasi, terverifikasi, reset password)
   └─ /api/ (lainnya)        ─► backend (Gin) :8080 ─► PostgreSQL (qms_db)

PostgreSQL ─ pg_notify('queue_events') ─► notify-service :8091 ─► Web Push (browser) dan FCM (Android)
PostgreSQL ─ notification_outbox       ─► mail-service :8092 ─► SMTP (Mailpit di dev)
BorderPay ─ webhook payment.paid ─► nginx ─► payment-service :8095 ─► PostgreSQL (payment_rw)
backend ─ POST /internal/v1/payments (X-Service-Token) ─► payment-service (checkout langganan)
```

Hanya nginx yang membuka port ke host. `auth-service`, `notify-service`, dan `mail-service` berada di jaringan internal Docker.

### Lapisan backend

```
handler  →  service  →  repository  →  PostgreSQL (sqlx + pgx)
   │           │
   │           └── notifier (hub SSE)
   └── dto (bentuk JSON)
```

- **handler** membaca request dan memetakan error ke kode HTTP. Tidak berisi aturan bisnis.
- **service** berisi semua aturan bisnis: kepemilikan, langganan, urutan antrian, join ulang.
- **repository** berisi SQL dan tidak tahu aturan bisnis.
- **dto** adalah bentuk JSON ke klien; model database tidak pernah dikirim langsung.
- Setiap lapisan mendefinisikan interface yang dibutuhkannya sendiri, dan perakitannya manual di `cmd/server/main.go`. auth-service, notify-service, dan mail-service memakai pola berlapis yang sama (`cmd` hanya merangkai; `internal/` berisi apperror, config, model, dto, repository, service, handler, middleware, router, dan paket khusus per service). Akibatnya service bisa diuji dengan fake repository tanpa database.

### Real-time dan komunikasi antar-layanan

- **SSE**: satu hub di memori untuk seluruh backend, dengan satu *room* per antrian (layar publik punya room sendiri). Service memberi tahu hub setelah transaksi selesai (join, next, skip, update, close), lalu hub menyebar event ke klien di room antrian itu. Heartbeat tiap 25 detik. Klien memakai `fetch` stream agar header `Authorization` tetap dikirim.
- **notify-service**: trigger Postgres memancarkan `pg_notify`; service mendengarkan lalu mengirim Web Push (browser) dan FCM (aplikasi Android). Tidak ada panggilan langsung dari backend.
- **auth-service**: nginx meneruskan rute login, registrasi, Google, reset password, dan verifikasi email ke sini. Backend tidak memanggilnya; ia memverifikasi JWT RS256 secara lokal dengan kunci publik. Bila auth-service mati, user yang sudah login tetap bisa memakai aplikasi (nginx menjawab 503 `AUTH_UNAVAILABLE` untuk rute auth).
- **mail-service**: trigger menulis ke `notification_outbox` dalam transaksi yang sama dengan perubahan tiket atau penutupan antrian; worker mengirim dan mencoba ulang. Email auth (verifikasi, `email-verified`, reset password) dikirim auth-service lewat `POST /internal/v1/send`.
- **Langganan**: `POST /api/v1/owners/subscription/checkout` di backend memanggil payment-service (`/internal/v1/payments`, dengan token layanan). Pembayaran lunas diterapkan lewat webhook BorderPay.
- **Keamanan akses data**: tiap service memakai role database sendiri dengan hak minimal (`notify_ro`, `mail_rw`, `auth_rw`, `payment_rw`, dan `metrics_ro` untuk observability); backend memakai role aplikasi utama.

## 6. Trade-offs

| Keputusan | Manfaat | Harga yang dibayar |
|---|---|---|
| Hub SSE di memori | Sederhana, event langsung tanpa infrastruktur tambahan | Hanya satu instance backend; scale-out butuh pub/sub bersama |
| Dipecah menjadi service terpisah (auth, notify, mail, payment) | Hak database dibatasi per service, dan kegagalan terisolasi (auth mati, user yang sudah login tetap memakai antrian) | Lebih banyak container, token internal, role DB, dan konfigurasi |
| Push tanpa outbox, email dengan outbox | Push cepat dan sederhana | Push yang terjadi saat service mati hilang; email terjamin tetapi butuh tabel outbox, worker, dan backoff |
| Join ulang mengembalikan tiket yang ada, bukan error | Scan dua kali atau double-tap aman | Klien tidak lagi mendapat sinyal "sudah bergabung" saat join (`ALREADY_JOINED` tidak dikirim oleh join; hanya sisip oleh owner yang masih menjawab 409 `ALREADY_JOINED`) |
| `sort_key` dengan gap 65536 | Penyisipan di tengah tanpa menulis ulang baris lain | Nilai maksimum harus dihitung dari semua tiket, bukan hanya `waiting`, karena `UNIQUE (queue_id, sort_key)` (pernah jadi bug) |
| SSE lewat `fetch` stream | Header `Authorization` tetap dikirim, token tidak masuk URL | Tidak memakai `EventSource` bawaan browser |
| 404 untuk akses yang tidak diizinkan | Keberadaan data orang lain tidak bocor | Pesan error tidak membedakan "tidak ada" dari "bukan milik Anda" |
| Layar publik hanya menampilkan nomor | Privasi | Tidak ada nama customer di layar |
| Tanpa `WriteTimeout` di backend | Koneksi SSE berumur panjang tidak terputus | Batas tulis untuk rute biasa dipegang nginx (`proxy_read_timeout`), bukan backend |
| Verifikasi email membatasi aksi (bergabung, membuat antrian), bukan login | User yang salah ketik email tidak terkunci dan masih bisa membuka Profil untuk kirim ulang | Akun belum terverifikasi tetap bisa masuk dan melihat; status dibaca dari database tiap permintaan (bukan JWT), satu query tambahan pada join dan buat antrian |
| Rate limit login dan `/join` hanya di nginx | Satu tempat untuk membatasi, tanpa kode tambahan di backend | Backend yang dijangkau tanpa nginx tidak membatasi login |
| Interface didefinisikan di tiap lapisan | Service bisa diuji dengan fake tanpa database | Ada interface ganda yang tidak dipakai (mis. `database.TxRunnerInterface` dan `handler_interface.go`) |
| Pembayaran QRIS lewat BorderPay dengan service terpisah `payment-service` | Pembayaran sungguhan. Langganan hanya berubah lewat webhook yang diverifikasi token dan idempoten. Service pembayaran bisa mati tanpa menghentikan antrian | Satu service dan satu role database tambahan yang harus dijaga (grant, token, webhook). Pembayaran gagal bila WAF memblokir webhook |
| Service pembayaran BorderPay ditambahkan kembali, menggantikan keputusan awal tanpa pembayaran | Pembayaran nyata; konfigurasi dan rahasia diperiksa sebelum deploy (`check-payment-config.sh`) | Lebih banyak container, rahasia, dan role database yang harus dijaga |

## 7. Next steps

1. **Push notification untuk android**: buat sistem push notification android.
2. **Backup otomatis** dengan jadwal, retensi, salinan di luar mesin, dan role database ikut tercakup.
3. **Kebersihan kode dan tes**: hapus `handler_interface.go` dan `database.TxRunnerInterface` yang tidak dipakai.
4. **Plan langganan tambahan** (2 bulan, 3 bulan, 6 bulan, 12 bulan) dan penagihan berulang bila dibutuhkan produk.
5. **Aplikasi ios**: agar bisa di download oleh user ios.