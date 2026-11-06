# Queue Management System (Sistem Antrian)

Sistem antrian untuk layanan tatap muka. Owner membuat antrian dan memanggil nomor berikutnya. Customer bergabung
lewat tautan dan melihat posisi serta nomor yang sedang dilayani secara real-time, tanpa perlu refresh.

## Alur pengguna

**Owner**
1. Daftar atau masuk, lalu buat antrian baru. Membuat antrian butuh langganan aktif.
2. Di detail antrian, bagikan QR atau tautan join ke customer. QR statis berisi kode tetap yang bisa diganti; QR dinamis kodenya berubah berkala.
3. Di Dashboard, tekan **Next** untuk memanggil nomor berikutnya. **Skip** melewati nomor yang sedang dilayani, dan **Panggil ulang** mengulang panggilan.
4. Tutup antrian saat layanan selesai. Customer yang belum bergabung tidak bisa join lagi.

**Customer**
1. Scan QR atau buka tautan join. Login dulu bila belum masuk, lalu kembali ke halaman join.
2. Tekan Gabung untuk mendapat nomor antrian. Scan berulang tidak membuat tiket kedua.
3. Pantau posisi dan nomor yang sedang dilayani di Beranda atau Antrian Saya. Notifikasi "Giliran Anda" datang saat nomor dipanggil.

Diagram dan detail lengkap ada di [FLOW.md](docs/FLOW.md).

Dokumen lain:
- [DESIGN.md](docs/DESIGN.md): arsitektur, keputusan desain, dan batasan.
- [FLOW.md](docs/FLOW.md): alur pengguna dan diagram urutan.
- [api-contract.md](docs/api-contract.md): kontrak HTTP dan SSE.
- [deploy/nginx/README.md](deploy/nginx/README.md): pemasangan nginx.

## Fitur yang ada

| Fitur | Penjelasan singkat |
|---|---|
| Akun | Daftar, masuk dengan email atau username, keluar, profil, avatar, dan preferensi. Lupa dan reset password lewat email bila mail-service dipasang. |
| Antrian | Owner membuat antrian (butuh langganan aktif), mengubah nama dan deskripsi, dan menutup antrian. |
| Panggilan nomor | Owner memakai Next, Skip (dengan aturan nomor yang dilewati), dan Panggil ulang. Customer bisa membatalkan tiketnya, dan owner bisa menghapus tiket. |
| QR dan tautan join | QR statis atau dinamis dengan token dari `qr-service`, penggantian kode statis, dan salin tautan join. Customer hanya bisa join lewat token. |
| Customer | Bergabung lewat QR atau tautan, melihat nomor yang sedang dilayani dan posisi di depan, serta riwayat tiket. |
| Catatan | Owner menulis catatan untuk nomor yang sedang dilayani. |
| Counter | API untuk menambah, mengganti nama, dan menghapus counter per antrian. Di UI saat ini hanya ada satu counter default. |
| Layar publik | Owner mengaktifkan layar di `/display/<token>` untuk monitor ruang tunggu, tanpa login. Satu kartu per counter, langsung berubah saat Next atau Skip. "GANTI TAUTAN" mematikan tautan lama. |
| Notifikasi giliran | Web Push dari `notify-service`: "Giliran Anda" saat dipanggil, "Tinggal 1 nomor lagi" untuk nomor berikutnya, dan pemberitahuan untuk nomor yang dilewati. Aktif otomatis saat join bila izin diberikan. Hanya di browser lewat HTTPS (`make tunnel`). |
| Email | `mail-service` mengirim email dari outbox dan mencoba ulang bila gagal. Pengguna bisa mematikannya di Profil. |
| Riwayat dan ekspor | Owner melihat riwayat antrian dan bisa mengekspornya ke CSV atau XLSX. Customer melihat riwayat tiketnya. |
| Langganan | Owner memperpanjang langganan 1 bulan lewat Stripe (`payment-service`). |
| Real-time | Pembaruan lewat Server-Sent Events, termasuk `queue_qr_updated`. |
| Tampilan | Mode terang dan gelap. |

Yang belum tersedia (lihat [DESIGN.md](docs/DESIGN.md#7-batasan-dan-yang-belum-dibangun)): login Google.

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
├── db/                       Dockerfile, migrations (skema, indeks, role per service), dan scripts
├── deploy/nginx/             konfigurasi nginx (reverse proxy, rate limit)
├── scripts/                  backup-prod.sh dan create-demo-accounts.sh
├── docker-compose.yml        stack development (hot reload)
├── docker-compose.prod.yml   stack demo/produksi (nginx hanya di 127.0.0.1)
├── Makefile                  perintah umum: make up, make tunnel, make prod-up, dan lainnya
├── .env.example              template env dev
└── .env.prod.example         template env produksi
```

Dokumen desain (DESIGN.md, FLOW.md, api-contract.md, dan lainnya) berada di folder `documentation/docs`, di luar folder ini.

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

Tunnel `qms-ta` adalah service systemd yang **manual**: `sudo systemctl start cloudflared` sebelum demo, `sudo systemctl stop cloudflared` sesudahnya (tidak hidup saat boot, berhenti otomatis saat laptop mati). Akses dibatasi aturan WAF Cloudflare berdasarkan IP WiFi. Semua perintah tanpa `make` dan `scripts/` ada di [docs/setup-perintah-manual.md](docs/setup-perintah-manual.md); panduan untuk mentor di [docs/petunjuk-mentor.md](docs/petunjuk-mentor.md).

Nginx hanya terikat ke `127.0.0.1:8080`; `cloudflared` di host menunjuk ke `http://localhost:8080`. IP klien dibaca dari `CF-Connecting-IP`. Backend dengan `APP_ENV=production` menolak start bila rahasia lemah atau origin bukan https. Database prod memakai volume sendiri tanpa seed. Backend berjalan satu instance (hub SSE di memori). Lihat `docs/setup-domain.md`, `docs/setup-waf-wifi.md`, dan `docs/plan-live.md`.

## Notifikasi (notify-service)

Variabel `.env` (lihat `.env.example`): `NOTIFY_DB_PASSWORD` (password role `notify_ro`, hanya baca), `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBJECT`. Buat kunci VAPID dengan `npx web-push generate-vapid-keys`.

Alur: trigger `tickets_notify_trg` di Postgres memancarkan `pg_notify('queue_events')` saat nomor dipanggil atau dipanggil ulang; `notify-service` mendengarkan, menghitung penerima, lalu mengirim push. Tanpa outbox, jadi push yang terjadi saat service mati hilang.

Catatan:
- Satu browser hanya melayani satu akun. Layar tiket mendaftarkan ulang langganan ke akun yang sedang login, dan logout melepas langganan.
- Push hanya jalan di HTTPS (domain tunnel `qms.montechristo.my.id`, atau `make tunnel` untuk uji sementara) dan di browser yang mendukung; Expo Go tidak. iPhone/iPad perlu situs dipasang ke Layar Utama.
- DB yang sudah berjalan tidak menjalankan ulang skrip init. **Cara cepat dan idempoten:** `docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql` (prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod`), lalu restart backend dan notify-service. Isinya mencakup semua penyesuaian di bawah ini, yang bila dikerjakan manual: `ALTER TABLE tickets ADD COLUMN recalled_at TIMESTAMPTZ;`, tabel `push_subscriptions`, fungsi `tickets_notify` dan trigger `tickets_notify_trg` (jika sudah ada versi lama tanpa event `skipped` atau `requeued`, jalankan ulang `CREATE OR REPLACE FUNCTION` dan buat ulang trigger) (ada di akhir `db/migrations/01_Schema.sql`), serta role `notify_ro` (`db/migrations/03_notify_role.sh`). Pengaturan Skip juga butuh kolom `queue_settings.skip_fallback`, `skip_fallback_n`, `skip_max_returns`.
- Awalan nomor tiket: DB lama butuh `ALTER TABLE queues ADD COLUMN ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A' CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$');`, lalu restart backend dan notify-service (notify membaca kolom ini).
- Tes unit: `cd notify-service && go test ./...`.

## Pembayaran langganan (payment-service)

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

## Email (Mailpit atau SMTP nyata)

`mail-service` adalah satu-satunya pengirim email (`notify-service` hanya Web Push dan tetap jalan walau email mati; dijaga `scripts/check-service-isolation.sh`). Pemanggil memakai `POST /internal/v1/send` dengan header `X-Service-Token` (`MAIL_SERVICE_TOKEN`). Pengaturan SMTP lewat `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_TLS` (`none`, `starttls`, atau `tls`); divalidasi saat start dan service menolak start bila salah atau tidak lengkap.

### Email notifikasi tiket (outbox)
Empat email ke pemilik tiket (masuk antrian, dibatalkan, nomor dihapus owner, selesai dilayani) ditulis ke tabel `notification_outbox` oleh trigger dalam transaksi yang sama dengan perubahan tiket, lalu dikirim oleh worker di `mail-service` (role database `mail_rw`, `MAIL_DB_PASSWORD`). Email yang gagal dicoba ulang dengan jeda berlipat; bila mail-service atau SMTP mati, email menunggu di outbox dan terkirim setelah hidup lagi, sementara join, cancel, dan push tidak terpengaruh. Pengguna bisa mematikannya di Profil (`Notifikasi email`).

**DB yang sudah berjalan** (skrip init hanya jalan pada data directory kosong): terapkan skema dan buat role:
```
docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
docker compose exec -T -e MAIL_DB_PASSWORD="$(grep ^MAIL_DB_PASSWORD .env | cut -d= -f2)" db sh < db/migrations/04_mail_role.sh
docker compose up -d mail-service
```
(prod: tambahkan `-f docker-compose.prod.yml --env-file .env.prod` dan ambil `MAIL_DB_PASSWORD` dari `.env.prod`). Tes: `QMS_DOCKER_EXEC=1 npx playwright test ts-30` (dua tes ketahanan mematikan dan menyalakan container).

### Link Mailpit
Mailpit hanya diikat ke `127.0.0.1` (UI `:8025`, SMTP `:1025`) dan **tidak lewat nginx**, jadi tidak terbuka lewat IP LAN maupun tunnel Cloudflare. Perangkat lain memakai SSH tunnel: `ssh -L 8025:127.0.0.1:8025 <user>@<IP mesin>`, lalu `http://localhost:8025`. Port diubah lewat `MAILPIT_UI_PORT` dan `MAILPIT_SMTP_PORT`. `make mailpit` menampilkan semua link tanpa menjalankan apa pun.

| Mode | Perintah | Email pergi ke | Link Mailpit |
|---|---|---|---|
| Dev (LAN) | `make up` | Mailpit (tidak keluar ke internet) | `http://localhost:8025` (dicetak di akhir `make up`) |
| Dev + tunnel | `make tunnel` | Mailpit | sama; tidak ikut lewat tunnel. Tautan di email memakai URL tunnel sesi itu |
| Prod, SMTP nyata | `make prod-up` (`MAIL_MODE=smtp`, bawaan) | Penerima sebenarnya | Tidak ada Mailpit; lihat inbox penerima atau dashboard penyedia SMTP |
| Prod, Mailpit | `make prod-up-mailpit` (atau `MAIL_MODE=mailpit`) | Mailpit, **tidak terkirim ke siapa pun** | `http://localhost:8025` di server prod; dari jauh pakai SSH tunnel |

### Memilih mode email produksi
`MAIL_MODE` di `.env.prod` (`smtp` atau `mailpit`), atau satu kali jalan dari baris perintah: `make prod-up MAIL_MODE=mailpit` / `make prod-up MAIL_MODE=smtp` (menimpa file). Mode `mailpit` menyalakan container Mailpit lewat compose profile `mailpit` dan memaksa `SMTP_*` ke Mailpit (menimpa isi `.env.prod`, jadi kredensial SMTP nyata tidak dipakai). Pesan Mailpit prod disimpan di volume `mailpit_data_prod` (bertahan saat restart). Pindah ke `smtp` menghentikan container Mailpit; `make prod-down` menghentikannya di kedua mode. `scripts/check-mail-mode.sh` dijalankan oleh `make prod-up` dan menolak: mode selain `smtp`/`mailpit`, `SMTP_*` kosong atau masih berisi `GANTI`, `SMTP_HOST=mailpit` pada mode `smtp`, `SMTP_TLS=none` ke host jauh, dan `MAIL_SERVICE_TOKEN` atau `MAIL_DB_PASSWORD` kosong, contoh, atau (token) kurang dari 32 karakter. Menjalankan `docker compose -f docker-compose.prod.yml` langsung (tanpa make) pada mode `mailpit` butuh `--profile mailpit` dan `SMTP_HOST=mailpit SMTP_PORT=1025 SMTP_TLS=none` di lingkungan.

### Dev dan uji
- Compose dev sudah mengarahkan ke `mailpit:1025`. Pesan hilang saat container dibuat ulang. Uji jalur email tanpa menunggu peristiwa antrian: `docker compose exec mail-service go run ./cmd/mail-service send-test alamat@contoh.com`.
- **Produksi SMTP nyata:** `docker-compose.prod.yml` **mewajibkan** `MAIL_SERVICE_TOKEN`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM` (lihat `.env.prod.example`). Gunakan `starttls` (587) atau `tls` (465) dengan kredensial dari penyedia dan alamat pengirim dari domain yang diverifikasi (SPF/DKIM).
- **Tes Playwright:** helper `playwright/unit-test/support/mailpit.ts` (`waitForMail`, `mailsTo`, `expectNoMail`, `uniqueRecipient`) membaca API Mailpit (`QMS_MAILPIT_URL`, bawaan `http://localhost:8025`); tes email di-skip bila Mailpit tidak menjawab. Pakai penerima unik per tes karena kotak surat dibagi semua tes.
- Memakai image baru? Pin versi di `docker-compose.yml` dan `docker-compose.prod.yml`; pull dari jaringan yang memeriksa sertifikat (proxy) bisa gagal, ulangi dari jaringan lain.

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
