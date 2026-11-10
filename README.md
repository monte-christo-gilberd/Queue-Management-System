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

## 1. Overview and assumptions

### Alur pengguna

**Owner**
1. Daftar, verifikasi email lewat tautan yang dikirim (lihat catatan di bawah), lalu buat antrian baru. Membuat antrian butuh email terverifikasi dan langganan aktif.
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
- Join hanya lewat token QR atau tautan. Token statis bisa diganti owner, token dinamis kedaluwarsa.
- Owner butuh langganan aktif untuk membuat antrian. User biasa menjadi owner lewat pembayaran pertama.
- Layar publik hanya menampilkan nomor, tanpa nama customer (privasi).
- Riwayat dan antrian tidak pernah dihapus permanen (keputusan produk 31 Okt 2026).
- Backend berjalan satu instance, karena hub SSE ada di memori proses.
- Frontend dan API satu origin lewat nginx. `PUBLIC_APP_URL` harus bisa dibuka dari HP yang memindai QR.
- Web Push butuh HTTPS dan browser yang mendukung.

### Status

Proyek capstone. Hasil pembacaan seluruh kode: backend, frontend, dan keempat service sudah terhubung ujung ke ujung, tanpa stub dan tanpa TODO di backend maupun service. Satu-satunya fitur yang belum tersambung adalah login Google. Sisanya adalah ketahanan dan operasional (lihat [bagian 4](#4-what-was-deferred-and-why) dan [bagian 7](#7-next-steps)).

### Dokumen lain

- [DESIGN.md](docs/DESIGN.md): arsitektur, keputusan desain, dan batasan.
- [FLOW.md](docs/FLOW.md): alur pengguna dan diagram urutan.
- [api-contract.md](docs/api-contract.md): kontrak HTTP dan SSE.
- [deploy/nginx/README.md](deploy/nginx/README.md): pemasangan nginx.

## 2. Setup and run

Dua jalur: **lokal** (tiap service dijalankan manual) atau **Docker Compose** (seluruh stack sekaligus, lebih mudah). Bagian di bawah berurutan: lokal, tes, Docker Compose, demo/produksi, build frontend, lalu konfigurasi layanan pendukung.

### Menjalankan secara lokal

#### 1. Database

Buat database, lalu terapkan skema:

```bash
createdb qms_db
for f in db/migrations/01*.sql db/migrations/02_Indexes.sql; do psql -d qms_db -v ON_ERROR_STOP=1 -f "$f"; done
```

Skema dipecah per domain (`01a_accounts.sql` sampai `01h_role_grants.sql`), lalu `02_Indexes.sql`; urutan nama = urutan ketergantungan. Peta file dan aturan menambah atau mengubah skema ada di [db/migrations/README.md](db/migrations/README.md).

#### 2. qr-service

```bash
cd qr-service
# Memakai ../.env (akar proyek): bagian [qr-service] di sana, isi JOIN_LINK_SECRET (>=32 byte) + QR_SERVICE_TOKEN
go run ./cmd/qr-service  # http://localhost:8090
```

`JOIN_LINK_SECRET` hanya dipegang qr-service. Backend memanggil endpoint internal
`/internal/v1/join-token` dan `/internal/v1/join-token/verify` dengan header `X-Service-Token`.

#### 3. Backend

```bash
cd backend
# Memakai ../.env (akar proyek): bagian [backend] di sana, isi BACKEND_DATABASE_URL, JWT_SECRET, BACKEND_PORT, ALLOWED_ORIGINS + variabel QR
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
| `EMAIL_VERIFICATION_REQUIRED` | Verifikasi email. Kosong = nyala bila `MAIL_SERVICE_URL` terisi; `false` mematikan (semua akun dianggap terverifikasi); `true` tanpa mail-service ditolak saat start |
| `SHUTDOWN_TIMEOUT` | Detik menunggu request selesai saat backend berhenti (default 8; compose dev 2). Harus lebih kecil dari `stop_grace_period` |

Catatan: `PUBLIC_APP_URL` harus bisa dibuka dari HP yang memindai QR (jangan `localhost` untuk uji perangkat).

#### 4. Frontend

```bash
cd frontend
npm install
npm run web          # http://localhost:8081
npm run web:lan      # bisa diakses dari perangkat lain di jaringan yang sama
```

Alamat API diatur lewat `EXPO_PUBLIC_API_URL`. Kalau kosong, frontend memakai host yang dibuka dengan port 8080.

### Menjalankan tes

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

### Docker Compose

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
| notify-service | `notify-service/Dockerfile` | Target `dev`, hanya di jaringan internal. LISTEN ke Postgres (role `notify_ro`), kirim Web Push (VAPID). Tidak bergantung pada email |
| mail-service | `mail-service/Dockerfile` | Target `dev`, hanya di jaringan internal (:8092). Satu-satunya pengirim email: API `POST /internal/v1/send` (`X-Service-Token`), templat, SMTP, coba ulang |
| payment-service | `payment-service/Dockerfile` | Target `dev`, jaringan internal (:8093), diteruskan nginx hanya di `/api/v1/payments/`. Perpanjang langganan owner 1 bulan setelah pembayaran Stripe (role DB `payment_rw`). Terpisah dari antrian: lihat bagian Pembayaran |
| mailpit | `axllent/mailpit:v1.31.6` | Hanya dev. Server SMTP palsu: menangkap semua email dari mail-service. SMTP dan UI hanya diikat ke 127.0.0.1 (`:1025` dan `:8025`), tidak lewat nginx; mail-service memakai `mailpit:1025` lewat jaringan internal |
| backend | `backend/Dockerfile` | Target `dev` (`go run`, restart manual), healthcheck `/health`, hanya di jaringan internal |
| frontend | `frontend/Dockerfile` | Target `dev` (Metro, hot reload) di port 80 internal |
| nginx | `deploy/nginx/Dockerfile` | Reverse proxy, satu-satunya yang membuka port (80) |

Satu `docker-compose.yml` untuk development; semua kode di-mount. Frontend hot reload; Go perlu `docker compose restart backend` (`qr-service`, `notify-service`) setelah ubah kode. Untuk demo/produksi pakai `docker-compose.prod.yml` (lihat bawah).
Kredensial (`POSTGRES_PASSWORD`, `JWT_SECRET`, `JOIN_LINK_SECRET`, `QR_SERVICE_TOKEN`) tidak dibakar ke image.
Frontend memanggil API di origin halaman (`EXPO_PUBLIC_API_URL=origin`), jadi mengganti `QMS_PUBLIC_ORIGIN` tidak butuh build ulang frontend.

Untuk uji dari HP, jalankan `make up` (build + stack, alamat memakai IP LAN) atau `make tunnel` (HTTPS publik, kamera HP jalan); `make down` untuk berhenti. Perintah lain: `make restart` (backend, qr-service, notify-service), `make seed`, `make logs`. Lihat `Makefile` dan `deploy/nginx/README.md`.

### Demo/produksi di belakang Cloudflare Tunnel

```bash
cp .env.prod.example .env.prod && chmod 600 .env.prod   # isi rahasia (openssl rand -hex 32) dan kunci VAPID baru; .env dev tidak tersentuh
make down                   # stack dev memakai port 8080 yang sama
make prod-up                # docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
scripts/create-demo-accounts.sh   # 1 owner + 4 customer (sudah terverifikasi emailnya), sandi acak di ~/qms-demo-accounts.txt
make prod-guard             # setelah akun demo dibuat: seed/reset ditolak di DB ini
make prod-backup            # ~/qms-backups/qms-<waktu>.sql.gz; restore: scripts/backup-prod.sh restore <file>
make prod-reset             # kosongkan antrian/tiket/push untuk demo ulang; akun dipertahankan, backup otomatis dulu
```

Tunnel `qms-ta` adalah service systemd yang **manual**: `sudo systemctl start cloudflared` sebelum demo, `sudo systemctl stop cloudflared` sesudahnya (tidak hidup saat boot, berhenti otomatis saat laptop mati). Akses dibatasi aturan WAF Cloudflare berdasarkan IP WiFi. Semua perintah tanpa `make` dan `scripts/` ada di [docs/setup-perintah-manual.md](docs/setup-perintah-manual.md); panduan untuk mentor di [docs/petunjuk-mentor.md](docs/petunjuk-mentor.md).

Nginx hanya terikat ke `127.0.0.1:8080`; `cloudflared` di host menunjuk ke `http://localhost:8080`. IP klien dibaca dari `CF-Connecting-IP`. Backend dengan `APP_ENV=production` menolak start bila rahasia lemah atau origin bukan https. Database prod memakai volume sendiri tanpa seed. Backend berjalan satu instance (hub SSE di memori). Lihat `docs/setup-domain.md`, `docs/setup-waf-wifi.md`, dan `docs/plan-live.md`.

### Daftar script

Semua script berada di `scripts/` (dijalankan dari akar `queue-management-system/`) dan `db/scripts/`. Sebagian besar dipanggil lewat `make`; yang lain dijalankan langsung. Tabel ini menjelaskan kapan masing-masing dipakai; penjelasan rinci per skrip ada di [scripts/README.md](scripts/README.md).

| Script | Fungsi | Cara pakai | Dipanggil dari |
|---|---|---|---|
| `scripts/backup-prod.sh` | Backup (`pg_dump`, gzip) dan restore database prod. Restore **menimpa** data prod. | `make prod-backup`, atau `QMS_ENV_FILE=.env.prod scripts/backup-prod.sh backup`. Restore: `scripts/backup-prod.sh restore <file.sql.gz>` | `make prod-backup`, `make prod-reset` |
| `scripts/reset-demo.sh` | Mengosongkan antrian, tiket, catatan, riwayat, dan langganan push di prod. Akun dipertahankan. Backup otomatis dibuat dulu. | `make prod-reset`, atau `scripts/reset-demo.sh -y` untuk melewati konfirmasi | `make prod-reset` |
| `scripts/create-demo-accounts.sh` | Membuat 1 owner (langganan 30 hari) dan 4 customer demo dengan sandi acak. Daftar akun ditulis ke `~/qms-demo-accounts.txt`. Tidak menimpa berkas yang sudah ada. | `scripts/create-demo-accounts.sh` (stack prod harus sudah berjalan) | Manual |
| `scripts/verify-user.sh` | Menandai satu email sebagai terverifikasi langsung di database. Dipakai bila email verifikasi tidak sampai. | `make verify-user EMAIL=<email> [STACK=prod]`, atau `scripts/verify-user.sh <email> [prod]` | `make verify-user` |
| `scripts/check-mail-mode.sh` | Memeriksa `MAIL_MODE` dan `SMTP_*` di `.env.prod` sebelum stack prod naik. Menolak konfigurasi yang salah. | Otomatis lewat `make prod-up`, atau `MAIL_MODE=smtp scripts/check-mail-mode.sh .env.prod` | `make prod-up` |
| `scripts/gen-auth-keys.sh` | Membuat pasangan kunci RSA untuk token login (`keys/`, tidak ikut git). Idempoten; tidak menimpa kunci yang ada. | Otomatis lewat `make up` dan `make prod-up` | `make up`, `make prod-up` |
| `scripts/check-service-isolation.sh` | Memastikan `notify-service` tidak bergantung pada mail-service, Mailpit, atau SMTP (dev dan prod), `payment-service` dan `auth-service` terpisah dari jalur antrian, dan kunci privat JWT hanya di-mount ke `auth-service`. Membutuhkan `docker` dan `python3`. | `scripts/check-service-isolation.sh` (jalankan setelah mengubah `docker-compose*.yml`) | Manual, tidak ada di Makefile |
| `db/scripts/apply_schema.sql` | Penyesuaian skema yang idempoten untuk database yang sudah berjalan. Tidak menyentuh data. | Dev: `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql`. Prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod`. Lalu restart backend dan notify-service. | Manual (lihat [Notifikasi](#notifikasi-notify-service) dan [Email](#email-mailpit-atau-smtp-nyata)) |
| `db/scripts/reset_seed_dev.sql` | Mengembalikan database dev ke data seed. Ditolak di prod setelah `make prod-guard`. | `make seed` | `make seed`, tes Playwright |
| `db/scripts/backfill_currently_served.sql` | Backfill sekali jalan: tiket `completed` terakhir di antrian aktif menjadi `currently_served`. Berakhir dengan `ROLLBACK`: periksa jumlah baris, lalu ganti ke `COMMIT`. Jalankan setelah backup dan `apply_schema.sql`. | Dev: `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/backfill_currently_served.sql` | Manual |
| `db/migrations/*.sh` (`03`, `04`, `05`) | Membuat role database `notify_ro`, `mail_rw`, dan `payment_rw`. | **Tidak dijalankan manual.** Docker menjalankannya saat volume database masih kosong (lihat `db/Dockerfile`). | Docker `initdb` |

**Script yang sudah dihapus**

- `frontend/scripts/reset-project.js` (dan script npm `reset-project`) dihapus. Itu sisa template Expo yang memindahkan `src/` dan `scripts/` ke `example/`, dan tidak dipakai di proyek ini.

### Build frontend untuk produksi

```bash
cd frontend
EXPO_PUBLIC_API_URL=https://domain-anda npx expo export --platform web --clear
```

Hasilnya ada di `frontend/dist/`. Salin ke folder yang dibaca nginx, lalu ikuti [deploy/nginx/README.md](deploy/nginx/README.md).

### Konfigurasi layanan pendukung

Detail konfigurasi dan operasi untuk notify-service, payment-service, dan mail-service.

#### Notifikasi (notify-service)

Variabel `.env` (lihat `.env.example`): `NOTIFY_DB_PASSWORD` (password role `notify_ro`, hanya baca), `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`. Buat kunci VAPID dengan `npx web-push generate-vapid-keys`.

Alur: trigger `tickets_notify_trg` di Postgres memancarkan `pg_notify('queue_events')` saat nomor dipanggil atau dipanggil ulang; `notify-service` mendengarkan, menghitung penerima, lalu mengirim push. Tanpa outbox, jadi push yang terjadi saat service mati hilang.

Catatan:
- Satu browser hanya melayani satu akun. Layar tiket mendaftarkan ulang langganan ke akun yang sedang login, dan logout melepas langganan.
- Push hanya jalan di HTTPS (domain tunnel `qms.montechristo.my.id`, atau `make tunnel` untuk uji sementara) dan di browser yang mendukung; Expo Go tidak. iPhone/iPad perlu situs dipasang ke Layar Utama.
- DB yang sudah berjalan tidak menjalankan ulang skrip init. **Cara cepat dan idempoten:** `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql` (prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod`), lalu restart backend dan notify-service. Isinya mencakup semua penyesuaian di bawah ini, yang bila dikerjakan manual: `ALTER TABLE tickets ADD COLUMN recalled_at TIMESTAMPTZ;`, tabel `push_subscriptions`, fungsi `tickets_notify` dan trigger `tickets_notify_trg` (jika sudah ada versi lama tanpa event `skipped` atau `requeued`, jalankan ulang `CREATE OR REPLACE FUNCTION` dan buat ulang trigger) (ada di `db/migrations/01g_triggers.sql`), serta role `notify_ro` (`db/migrations/03_notify_role.sh`). Pengaturan Skip juga butuh kolom `queue_settings.skip_fallback`, `skip_fallback_n`, `skip_max_returns`.
- Awalan nomor tiket: DB lama butuh `ALTER TABLE queues ADD COLUMN ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A' CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$');`, lalu restart backend dan notify-service (notify membaca kolom ini).
- Tes unit: `cd notify-service && go test ./...`.

#### Pembayaran langganan (payment-service)

Satu kemampuan saja: **memperpanjang langganan 1 bulan** setelah Stripe mengonfirmasi pembayaran (tidak ada plan lain, tidak ada penagihan berulang). **Pembayaran pertama user biasa sekaligus menjadikannya owner**: itu satu-satunya jalan menjadi owner dari aplikasi (Profil → kartu "Jadi Owner" → Subscription → bayar). Ada di stack dev dan prod (`docker-compose.prod.yml`) sebagai bawaan.

**Alur.** User menekan "Bayar & Jadi Owner" (atau "Perpanjang 1 Bulan" bila sudah owner) di layar Subscription, frontend memanggil `POST /api/v1/payments/checkout` (JWT yang sama dengan backend), payment-service membuka sesi Stripe Checkout dan mengembalikan `checkout_url`. Setelah bayar, **webhook** Stripe `POST /api/v1/payments/webhooks/stripe` (tanda tangan diverifikasi) memperpanjang dalam satu transaksi (upsert): `owners.subscription_expires_at = GREATEST(now(), subscription_expires_at) + 1 bulan` (dari tanggal berakhir bila masih aktif, dari sekarang bila sudah lewat; user yang belum owner mendapat baris `owners` baru dengan `now() + 1 bulan`) dan satu baris `payments` (`plan_type='monthly'`). Idempoten per event **dan** per sesi Checkout, jadi kiriman ulang Stripe tidak memperpanjang dua kali. Redirect browser hanya UX; sumber kebenaran adalah webhook.

**Terisolasi dari antrian.** Backend hanya *membaca* `owners.subscription_expires_at` (402 `SUBSCRIPTION_INACTIVE` bila lewat) dan tidak pernah memanggil payment-service; tidak ada `depends_on` dari/ke backend maupun nginx. Role database `payment_rw` hanya boleh membuat baris `owners` (hanya kolom `user_id` dan `subscription_expires_at`), mengubah kolom langganan itu, membaca `users.email`, menambah `payments`, dan memakai dua tabel miliknya (`payment_webhook_events`, `payment_session_credits`); tidak ada akses ke antrian, tiket, atau `password_hash`. Bila service mati: login, antrian, tiket, SSE tetap jalan; checkout/webhook dijawab nginx dengan JSON 503 `PAYMENT_UNAVAILABLE` dan Stripe mengirim ulang webhook, sehingga perpanjangan terkejar setelah service hidup. nginx memakai upstream lewat variabel + resolver, jadi tetap bisa start tanpa payment-service. Tanpa kunci Stripe service tetap hidup dan checkout menjawab 503; `make prod-up` tidak gagal karenanya. Dijaga `scripts/check-service-isolation.sh`.

**Simulasi di dev (tanpa Stripe).** `DEV_PAYMENT_SIMULATOR=true` di `.env` membuat tombol bayar langsung berhasil: payment-service memanggil jalur perpanjangan yang sama dengan webhook asli (owner dibuat bila belum ada, +1 bulan, baris `payments` tercatat), tanpa jaringan dan tanpa uang. Aman karena: tidak diteruskan oleh `docker-compose.prod.yml` (dijaga `scripts/check-service-isolation.sh`), service menolak start bila menyala bersama `APP_ENV=production`, dan menolak start bila kunci Stripe juga terisi (pilih salah satu). Setiap klik adalah pembayaran baru, jadi klik ulang memperpanjang 1 bulan lagi.

**Konfigurasi** (`.env` / `.env.prod`, contoh di `.env.example` dan `.env.prod.example`): `PAYMENT_DB_PASSWORD`, `STRIPE_API_KEY` (`sk_live_...` produksi, `sk_test_...` uji tanpa uang sungguhan), `STRIPE_WEBHOOK_SECRET` (`whsec_...`), opsional `STRIPE_PRICE_ID`, `SUBSCRIPTION_PRICE_AMOUNT` (satuan utama, bawaan 50000) dan `SUBSCRIPTION_CURRENCY` (bawaan `idr`), `STRIPE_API_BASE_URL` (kosong = api.stripe.com; isi hanya untuk emulator). Daftarkan endpoint webhook `https://<domain>/api/v1/payments/webhooks/stripe` di dashboard Stripe dengan event `checkout.session.completed` dan `checkout.session.async_payment_succeeded`. Dev tanpa dashboard: `stripe listen --forward-to localhost:8080/api/v1/payments/webhooks/stripe`.

**DB yang sudah berjalan** (skrip init hanya jalan pada data directory kosong): buat tabel dan role (idempoten; jalankan ulang bila role sudah pernah dibuat dari versi lama, karena hak `INSERT` pada `owners` baru ditambahkan), lalu bangun ulang nginx (konfigurasinya masuk ke image) dan jalankan service:
```
docker compose exec -T -e PAYMENT_DB_PASSWORD="$(grep ^PAYMENT_DB_PASSWORD .env | cut -d= -f2)" db sh < db/migrations/05_payment_role.sh
docker compose up -d --build payment-service nginx
```
(prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod` dan ambil `PAYMENT_DB_PASSWORD` dari `.env.prod`).

**Tes.** Unit: `cd payment-service && go test ./...`. Integrasi database (membuktikan perpanjangan, idempotensi, dan bahwa `payment_rw` tidak bisa menyentuh data antrian) butuh Postgres dengan skema dan skrip 05 yang sudah diterapkan: `PAYMENT_TEST_ADMIN_URL=<superuser> PAYMENT_TEST_DATABASE_URL=<payment_rw> go test ./...` (dilewati bila env kosong). Isolasi: `scripts/check-service-isolation.sh`.

#### Email (Mailpit atau SMTP nyata)

`mail-service` adalah satu-satunya pengirim email (`notify-service` hanya Web Push dan tetap jalan walau email mati; dijaga `scripts/check-service-isolation.sh`). Pemanggil memakai `POST /internal/v1/send` dengan header `X-Service-Token` (`MAIL_SERVICE_TOKEN`). Pengaturan SMTP lewat `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_TLS` (`none`, `starttls`, atau `tls`); divalidasi saat start dan service menolak start bila salah atau tidak lengkap.

##### Email notifikasi tiket (outbox)
Empat email ke pemilik tiket (masuk antrian, dibatalkan, nomor dihapus owner, selesai dilayani) ditulis ke tabel `notification_outbox` oleh trigger dalam transaksi yang sama dengan perubahan tiket, lalu dikirim oleh worker di `mail-service` (role database `mail_rw`, `MAIL_DB_PASSWORD`). Email yang gagal dicoba ulang dengan jeda berlipat; bila mail-service atau SMTP mati, email menunggu di outbox dan terkirim setelah hidup lagi, sementara join, cancel, dan push tidak terpengaruh. Pengguna bisa mematikannya di Profil (`Notifikasi email`).

**DB yang sudah berjalan** (skrip init hanya jalan pada data directory kosong): terapkan skema dan buat role:
```
docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
docker compose exec -T -e MAIL_DB_PASSWORD="$(grep ^MAIL_DB_PASSWORD .env | cut -d= -f2)" db sh < db/migrations/04_mail_role.sh
docker compose up -d mail-service
```
(prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod` dan ambil `MAIL_DB_PASSWORD` dari `.env.prod`). Tes: `QMS_DOCKER_EXEC=1 npx playwright test ts-30` (dua tes ketahanan mematikan dan menyalakan container).

##### Verifikasi email
Akun baru dikirimi tautan `/verify-email?token=...` (berlaku 24 jam, sekali pakai, maksimal 3 kiriman per jam per akun; yang disimpan hanya hash tokennya di `user_action_tokens`). Pengiriman di latar belakang: bila gagal, registrasi tetap berhasil dan user bisa kirim ulang dari halaman Profil. Akun yang belum terverifikasi tidak bisa bergabung ke antrian (nomor baru) atau membuat antrian (403 `EMAIL_NOT_VERIFIED`); status dibaca dari database, jadi berlaku tanpa login ulang. Menyala bila `EMAIL_VERIFICATION_REQUIRED` kosong atau `true` (bawaan: ikut adanya mail-service).

| Lingkungan | Email verifikasi | Cara memverifikasi |
|---|---|---|
| Dev (`make up`) | Ditangkap Mailpit | Buka `http://localhost:8025`, klik tautan di email |
| Prod `MAIL_MODE=smtp` (bawaan) | Dikirim ke inbox asli lewat SMTP, mis. Gmail (`smtp.gmail.com`, 587, `starttls`, kata sandi aplikasi) | Pengguna klik tautan di inbox-nya |
| Prod `MAIL_MODE=mailpit` | Ditangkap Mailpit, **tidak terkirim ke siapa pun** | Ambil tautan dari Mailpit (SSH tunnel), atau `make verify-user EMAIL=<email> STACK=prod` |

- `make verify-user EMAIL=<email> [STACK=prod]` (`scripts/verify-user.sh`) menandai satu akun terverifikasi langsung di database. Idempoten dan aman terhadap karakter aneh di email.
- `scripts/check-mail-mode.sh` (dijalankan `make prod-up`) memperingatkan bahwa pengguna baru tidak bisa memverifikasi di mode `mailpit`, kecuali `EMAIL_VERIFICATION_REQUIRED=false`.
- **DB yang sudah berjalan:** `db/scripts/apply_schema.sql` menambah kolom `users.email_verified_at`, memperluas CHECK `purpose`, dan **sekali saja** menandai semua akun yang sudah ada terverifikasi (agar tidak terkunci); menjalankannya ulang tidak memverifikasi akun baru. Lalu restart backend dan mail-service (templat `email-verify` ada di image mail-service). Seed dev dan akun demo dibuat sudah terverifikasi.
- Tes: `ts-33-email-verification.spec.ts` (butuh Mailpit dan owner berlangganan). Helper `registerAccount` di tes e2e memverifikasi akun lewat Mailpit dan menghapus email verifikasinya; beri `{ verify: false }` untuk akun yang belum terverifikasi.

##### Link Mailpit
Mailpit hanya diikat ke `127.0.0.1` (UI `:8025`, SMTP `:1025`) dan **tidak lewat nginx**, jadi tidak terbuka lewat IP LAN maupun tunnel Cloudflare. Perangkat lain memakai SSH tunnel: `ssh -L 8025:127.0.0.1:8025 <user>@<IP mesin>`, lalu `http://localhost:8025`. Port diubah lewat `MAILPIT_UI_PORT` dan `MAILPIT_SMTP_PORT`. `make mailpit` menampilkan semua link tanpa menjalankan apa pun.

| Mode | Perintah | Email pergi ke | Link Mailpit |
|---|---|---|---|
| Dev (LAN) | `make up` | Mailpit (tidak keluar ke internet) | `http://localhost:8025` (dicetak di akhir `make up`) |
| Dev + tunnel | `make tunnel` | Mailpit | sama; tidak ikut lewat tunnel. Tautan di email memakai URL tunnel sesi itu |
| Prod, SMTP nyata | `make prod-up` (`MAIL_MODE=smtp`, bawaan) | Penerima sebenarnya | Tidak ada Mailpit; lihat inbox penerima atau dashboard penyedia SMTP |
| Prod, Mailpit | `make prod-up-mailpit` (atau `MAIL_MODE=mailpit`) | Mailpit, **tidak terkirim ke siapa pun** | `http://localhost:8025` di server prod; dari jauh pakai SSH tunnel |

##### Memilih mode email produksi
`MAIL_MODE` di `.env.prod` (`smtp` atau `mailpit`), atau satu kali jalan dari baris perintah: `make prod-up MAIL_MODE=mailpit` / `make prod-up MAIL_MODE=smtp` (menimpa file). Mode `mailpit` menyalakan container Mailpit lewat compose profile `mailpit` dan memaksa `SMTP_*` ke Mailpit (menimpa isi `.env.prod`, jadi kredensial SMTP nyata tidak dipakai). Pesan Mailpit prod disimpan di volume `mailpit_data_prod` (bertahan saat restart). Pindah ke `smtp` menghentikan container Mailpit; `make prod-down` menghentikannya di kedua mode. `scripts/check-mail-mode.sh` dijalankan oleh `make prod-up` dan menolak: mode selain `smtp`/`mailpit`, `SMTP_*` kosong atau masih berisi `GANTI`, `SMTP_HOST=mailpit` pada mode `smtp`, `SMTP_TLS=none` ke host jauh, dan `MAIL_SERVICE_TOKEN` atau `MAIL_DB_PASSWORD` kosong, contoh, atau (token) kurang dari 32 karakter. Menjalankan `docker compose -f docker-compose.prod.yml` langsung (tanpa make) pada mode `mailpit` butuh `--profile mailpit` dan `SMTP_HOST=mailpit SMTP_PORT=1025 SMTP_TLS=none` di lingkungan.

##### Dev dan uji
- Compose dev sudah mengarahkan ke `mailpit:1025`. Pesan hilang saat container dibuat ulang. Uji jalur email tanpa menunggu peristiwa antrian: `docker compose exec mail-service go run ./cmd/mail-service send-test alamat@contoh.com`.
- **Produksi SMTP nyata:** `docker-compose.prod.yml` **mewajibkan** `MAIL_SERVICE_TOKEN`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` (lihat `.env.prod.example`). Gunakan `starttls` (587) atau `tls` (465) dengan kredensial dari penyedia dan alamat pengirim dari domain yang diverifikasi (SPF/DKIM).
- **Tes Playwright:** helper `playwright/unit-test/support/mailpit.ts` (`waitForMail`, `mailsTo`, `expectNoMail`, `uniqueRecipient`) membaca API Mailpit (`QMS_MAILPIT_URL`, bawaan `http://localhost:8025`); tes email di-skip bila Mailpit tidak menjawab. Pakai penerima unik per tes karena kotak surat dibagi semua tes.
- Memakai image baru? Pin versi di `docker-compose.yml` dan `docker-compose.prod.yml`; pull dari jaringan yang memeriksa sertifikat (proxy) bisa gagal, ulangi dari jaringan lain.

## 3. What was built

### Progres per komponen

| Komponen | Status | Catatan |
|---|---|---|
| Backend (Go, Gin) | Lengkap dan terhubung | Semua route punya handler, service, dan repository. Tanpa stub dan tanpa TODO. Sekitar 49 file tes (unit, handler, router, SSE, integrasi). |
| Frontend (Expo) | Lengkap dan terhubung | Semua layar memakai API nyata. Sekitar 159 unit test dan 217 tes Playwright (32 spec). Satu-satunya layar yang belum tersambung: tombol Google OAuth. |
| qr-service | Lengkap | Token join HMAC, verifikasi, dan render QR. `/health` dan `/ready`. |
| notify-service | Lengkap | LISTEN Postgres, Web Push, coba ulang, dan `/health`. Tanpa outbox. |
| mail-service | Lengkap | Outbox dengan coba ulang dan backoff, enam templat, SMTP. |
| payment-service | Lengkap | Stripe Checkout, webhook idempoten, dan simulator untuk dev. |
| Database | Lengkap | 16 tabel, trigger notifikasi dan outbox email, role terpisah per service. |
| Deployment | Berjalan | Compose dev dan prod, nginx dengan rate limit, tunnel Cloudflare manual, Makefile dan skrip bantu. |

### Fitur yang ada

| Fitur | Penjelasan singkat |
|---|---|
| Akun | Daftar, masuk dengan email atau username, keluar, profil, avatar, dan preferensi. Lupa dan reset password lewat email bila mail-service dipasang. |
| Verifikasi email | Tautan verifikasi saat registrasi, halaman publik `/verify-email`, banner dan tombol kirim ulang di Profil. Belum terverifikasi: tidak bisa bergabung ke antrian atau membuat antrian. Dev lewat Mailpit, prod lewat `MAIL_MODE`. |
| Antrian | Owner membuat antrian (butuh langganan aktif), mengubah nama dan deskripsi, dan menutup antrian. |
| Panggilan nomor | Owner memakai Next, Skip (dengan aturan nomor yang dilewati), dan Panggil ulang. Customer bisa membatalkan tiketnya, dan owner bisa menghapus tiket. |
| QR dan tautan join | QR statis atau dinamis dengan token dari `qr-service`, penggantian kode statis, dan salin tautan join. Customer hanya bisa join lewat token. |
| Customer | Bergabung lewat QR atau tautan, melihat nomor yang sedang dilayani dan posisi di depan, serta riwayat tiket. |
| Catatan | Owner menulis catatan untuk nomor yang sedang dilayani. |
| Counter | Owner menambah, mengganti nama, dan menghapus counter per antrian lewat UI (`CounterBar`). Layar publik menampilkan satu kartu per counter. |
| Layar publik | Owner mengaktifkan layar di `/display/<token>` untuk monitor ruang tunggu, tanpa login. Satu kartu per counter, langsung berubah saat Next atau Skip. "GANTI TAUTAN" mematikan tautan lama. |
| Notifikasi giliran | Web Push dari `notify-service`: "Giliran Anda" saat dipanggil, "Tinggal 1 nomor lagi" untuk nomor berikutnya, dan pemberitahuan untuk nomor yang dilewati. Aktif otomatis saat join bila izin diberikan. Hanya di browser lewat HTTPS (`make tunnel`). |
| Email | `mail-service` mengirim email dari outbox dan mencoba ulang bila gagal. Pengguna bisa mematikannya di Profil. |
| Riwayat dan ekspor | Owner melihat riwayat antrian dan bisa mengekspornya ke CSV atau XLSX. Customer melihat riwayat tiketnya. |
| Langganan | Owner memperpanjang langganan 1 bulan lewat Stripe (`payment-service`). |
| Real-time | Pembaruan lewat Server-Sent Events, termasuk `queue_qr_updated`. |
| Tampilan | Mode terang dan gelap. |
| Shutdown rapi | Backend menangani SIGINT/SIGTERM: koneksi SSE ditutup, request yang berjalan dituntaskan (paling lama `SHUTDOWN_TIMEOUT`), lalu database ditutup. `ReadHeaderTimeout` 5 dtk, `ReadTimeout` 30 dtk, `IdleTimeout` 120 dtk. |

Yang belum ada dan alasannya ada di [bagian 4](#4-what-was-deferred-and-why).

### Struktur folder

```
queue-management-system/
├── backend/                  API Go (Gin, sqlx)
│   ├── cmd/server/           titik masuk (main.go): merangkai semua komponen
│   └── internal/
│       ├── apperror/         error bersama (sentinel) untuk repository, service, dan handler
│       ├── config/           membaca dan memvalidasi env var
│       ├── database/         koneksi PostgreSQL dan pembungkus transaksi (WithTx)
│       ├── dto/              bentuk request dan response JSON
│       ├── export/           ekspor riwayat antrian ke CSV dan XLSX
│       ├── handler/          endpoint HTTP
│       ├── imageproc/        validasi, crop, dan perkecil foto profil
│       ├── mailclient/       klien HTTP ke mail-service
│       ├── middleware/       autentikasi JWT dan pencabutan sesi
│       ├── model/            struct data (User, Queue, Ticket, Counter)
│       ├── qrclient/         klien HTTP ke qr-service
│       ├── repository/       akses data ke PostgreSQL
│       ├── router/           pendaftaran rute HTTP
│       ├── service/          logika bisnis dan aturan antrian
│       ├── sse/              hub Server-Sent Events (di memori)
│       └── validator/        aturan validasi kustom, mis. no_html_tags
├── frontend/                 aplikasi Expo: src/app (rute), components, hooks, lib, store
├── qr-service/               microservice token join QR (HMAC), hanya jaringan internal
├── notify-service/           Web Push (LISTEN Postgres), /health :8091
├── mail-service/             pengirim email (outbox dan SMTP), :8092
├── payment-service/          perpanjang langganan lewat Stripe, :8093
├── db/                       Dockerfile, migrations (skema, indeks, role per service), dan scripts (apply_schema, reset_seed_dev, backfill)
├── deploy/nginx/             konfigurasi nginx (reverse proxy, rate limit)
├── scripts/                  backup-prod, reset-demo, create-demo-accounts, verify-user, check-mail-mode, check-service-isolation (lihat "Daftar script")
├── docker-compose.yml        stack development (hot reload)
├── docker-compose.prod.yml   stack demo/produksi (nginx hanya di 127.0.0.1)
├── Makefile                  perintah umum: make up, make tunnel, make prod-up, dan lainnya
├── .env.example              template env dev
└── .env.prod.example         template env produksi
```

Dokumen desain (DESIGN.md, FLOW.md, api-contract.md, dan lainnya) berada di folder `documentation/docs`, di luar folder ini.

## 4. What was deferred and why

Kolom "Pilihan?" menandai apakah penundaan itu keputusan yang tercatat (Ya) atau hanya belum dikerjakan (Belum).

| Item | Kondisi sekarang | Pilihan? | Alasan atau catatan |
|---|---|---|---|
| Backend lebih dari satu instance | Hub SSE di memori | Ya | Butuh pub/sub bersama (LISTEN/NOTIFY atau Redis). Demo memakai satu instance (`DESIGN.md` §2.7). |
| Outbox untuk Web Push | Push yang terjadi saat `notify-service` mati hilang | Ya | Email sudah memakai outbox; push dibuat lebih sederhana dulu. Tercatat di `plan-setelah-live.md`. |
| Migration runner | `db/migrations/01*.sql` hanya jalan pada data directory kosong; DB lama memakai `db/scripts/apply_schema.sql` yang idempoten | Ya | Cukup selama skema jarang berubah. Tercatat di `plan-setelah-live.md`. |
| HSTS dan Cloudflare Access | Belum aktif; TLS dipegang Cloudflare, nginx hanya HTTP | Ya | Menunggu HTTPS stabil dan Zero Trust siap (`plan-setelah-live.md`). |
| Plan langganan lain | Hanya perpanjangan 1 bulan, tanpa penagihan berulang | Ya | Disengaja sederhana (bagian Pembayaran). Kolom `plan_type` selain `monthly` belum dipakai. |
| Ekspor file dan push di aplikasi native | Hanya jalan di web | Ya | Web Push hanya di browser lewat HTTPS; Expo Go tidak mendukung. |
| `registration_limit`, batas harian QR, batas riwayat, penghapusan permanen | Kolom skema ada, kode tidak memakainya | Ya | Dihapus dari produk pada 31 Okt 2026. |
| Login Google | Tombol di layar masuk dan daftar hanya tampilan; kolom `google_id` sudah ada | Ya | Tidak diperlukan; login email/username sudah cukup untuk demo. Bila dikerjakan, butuh Google OAuth di backend (komentar TODO di `LoginScreen.tsx`), Tier 2 di `plan-setelah-live.md`. |
| Backup otomatis | Hanya manual lewat `make prod-backup`, tanpa jadwal, retensi, atau salinan di luar mesin | Ya | Tidak diperlukan; cukup untuk demo. |
| Tes render komponen frontend | Tidak ada; hanya unit test logika dan Playwright | Ya | Logika murni diuji unit dan layar diuji Playwright; render test butuh `jest-expo` plus mock Reanimated/router dengan nilai tambah kecil. |
| Cek MX atau allowlist domain email | Tidak ada; verifikasi hanya lewat tautan | Ya | Kepemilikan email terbukti dengan klik tautan; domain yang tidak ada gagal terkirim dan akunnya tetap belum terverifikasi. |

## 5. Architecture

### Teknologi

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.27, Gin, sqlx + pgx, JWT HS256, bcrypt |
| qr-service | Go 1.27, token join HMAC (internal, tidak dipublikasikan nginx) |
| notify-service | Go 1.27, LISTEN Postgres, Web Push (VAPID) |
| mail-service | Go 1.27, SMTP, outbox di Postgres |
| payment-service | Go 1.27, Stripe Checkout (perpanjangan langganan 1 bulan) |
| Database | PostgreSQL |
| Frontend | Expo SDK 57 (Expo Router, web static), React Native, NativeWind, Zustand |
| Deploy | Docker Compose, nginx (reverse proxy + rate limit login dan `/join`) |

### Gambaran sistem

```
Browser (Expo web / native)
   │  HTTP (REST, JSON) + SSE
   ▼
nginx ── frontend statis (dist)
   ├─ /api/             ─► backend (Gin) :8080 ─► PostgreSQL (qms_db)
   │                          ├─► qr-service :8090   (token join, HTTP internal)
   │                          └─► mail-service :8092 (email reset password, HTTP internal)
   └─ /api/v1/payments/ ─► payment-service :8093 ─► PostgreSQL (role payment_rw) + Stripe

PostgreSQL ─ pg_notify('queue_events') ─► notify-service :8091 ─► Web Push ke browser
PostgreSQL ─ notification_outbox       ─► mail-service :8092 ─► SMTP (Mailpit di dev)
```

Hanya nginx yang membuka port ke host. `qr-service`, `notify-service`, `mail-service`, dan `payment-service` berada di jaringan internal Docker.

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
- Setiap lapisan mendefinisikan interface yang dibutuhkannya sendiri, dan perakitannya manual di `cmd/server/main.go`. Akibatnya service bisa diuji dengan fake repository tanpa database.

### Real-time dan komunikasi antar-layanan

- **SSE**: tiap antrian punya satu hub di memori. Service memberi tahu hub setelah transaksi selesai (join, next, skip, update, close), lalu hub menyebar event ke klien di antrian itu. Heartbeat tiap 25 detik. Klien memakai `fetch` stream agar header `Authorization` tetap dikirim.
- **qr-service**: backend memanggil `/internal/v1/join-token` dan `/internal/v1/join-token/verify` dengan header `X-Service-Token`. Rahasia `JOIN_LINK_SECRET` hanya dipegang qr-service.
- **notify-service**: trigger Postgres memancarkan `pg_notify`; service mendengarkan lalu mengirim Web Push. Tidak ada panggilan langsung dari backend.
- **mail-service**: trigger menulis ke `notification_outbox` dalam transaksi yang sama dengan perubahan tiket; worker mengirim dan mencoba ulang.
- **payment-service**: terisolasi dari antrian. Backend hanya membaca `owners.subscription_expires_at` dan tidak pernah memanggil payment-service.
- **Keamanan akses data**: tiap service memakai role database sendiri dengan hak minimal (`notify_ro`, `mail_rw`, `payment_rw`).

## 6. Trade-offs

| Keputusan | Manfaat | Harga yang dibayar |
|---|---|---|
| Hub SSE di memori | Sederhana, event langsung tanpa infrastruktur tambahan | Hanya satu instance backend; scale-out butuh pub/sub bersama |
| Dipecah menjadi service terpisah (qr, notify, mail, payment) | Hak database dibatasi per service, dan kegagalan terisolasi (payment mati, antrian tetap jalan) | Lebih banyak container, token internal, role DB, dan konfigurasi |
| Push tanpa outbox, email dengan outbox | Push cepat dan sederhana | Push yang terjadi saat service mati hilang; email terjamin tetapi butuh tabel outbox, worker, dan backoff |
| Join ulang mengembalikan tiket yang ada, bukan error | Scan dua kali atau double-tap aman | Klien tidak lagi mendapat sinyal "sudah bergabung" (`ALREADY_JOINED` tidak dikirim) |
| `sort_key` dengan gap 65536 | Penyisipan di tengah tanpa menulis ulang baris lain | Nilai maksimum harus dihitung dari semua tiket, bukan hanya `waiting`, karena `UNIQUE (queue_id, sort_key)` (pernah jadi bug) |
| SSE lewat `fetch` stream | Header `Authorization` tetap dikirim, token tidak masuk URL | Tidak memakai `EventSource` bawaan browser |
| 404 untuk akses yang tidak diizinkan | Keberadaan data orang lain tidak bocor | Pesan error tidak membedakan "tidak ada" dari "bukan milik Anda" |
| Layar publik hanya menampilkan nomor | Privasi | Tidak ada nama customer di layar |
| Tanpa `WriteTimeout` di backend | Koneksi SSE berumur panjang tidak terputus | Batas tulis untuk rute biasa dipegang nginx (`proxy_read_timeout`), bukan backend |
| Verifikasi email membatasi aksi (bergabung, membuat antrian), bukan login | User yang salah ketik email tidak terkunci dan masih bisa membuka Profil untuk kirim ulang | Akun belum terverifikasi tetap bisa masuk dan melihat; status dibaca dari database tiap permintaan (bukan JWT), satu query tambahan pada join dan buat antrian |
| Rate limit login dan `/join` hanya di nginx | Satu tempat untuk membatasi, tanpa kode tambahan di backend | Backend yang dijangkau tanpa nginx tidak membatasi login |
| Interface didefinisikan di tiap lapisan | Service bisa diuji dengan fake tanpa database | Ada interface ganda yang tidak dipakai (mis. `database.TxRunnerInterface` dan `handler_interface.go`) |
| Pembayaran satu jenis (perpanjang 1 bulan) | Alur dan idempotensi webhook sederhana | Tidak ada plan lain atau penagihan berulang |

## 7. Next steps

Urutan menurut dampak terhadap keandalan, dari yang paling dekat. Rencana lengkap ada di `docs/plan-setelah-live.md`.

1. **Perbaiki `scripts/reset-demo.sh`.** `TRUNCATE` di sana tidak menyertakan `queue_displays`, padahal tabel itu mereferensikan `queues` lewat foreign key. Postgres menolak truncate semacam itu, jadi reset demo kemungkinan gagal (belum dijalankan untuk memastikan).
2. **Login Google**: OAuth di backend, lalu sambungkan tombol yang sudah ada di frontend.
3. **Pub/sub untuk SSE** agar backend bisa lebih dari satu instance.
4. **Outbox untuk Web Push** dan worker pool terbatas di notify-service.
5. **Migration runner**, dan jadikan `03_notify_role.sh` idempoten seperti skrip role lain.
6. **Keamanan produksi**: HSTS setelah HTTPS stabil, Cloudflare Access, IP WiFi kantor di aturan WAF, dan `VAPID_SUBJECT` dengan email asli.
7. **Backup otomatis** dengan jadwal, retensi, salinan di luar mesin, dan role database ikut tercakup.
8. **Kebersihan kode dan tes**: hapus `handler_interface.go` dan `database.TxRunnerInterface` yang tidak dipakai; bersihkan 14 `test.fixme` usang di `ts-09-backlog-features.spec.ts`; tambah tes `note_handler`, `mailclient`, dan `database`; perbarui komentar usang di `docker-compose.yml` dan `deploy/nginx/README.md`.
9. **Plan langganan tambahan** bila dibutuhkan produk.
