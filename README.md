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
- Layar publik untuk monitor ruang tunggu: di detail antrian, owner menekan "AKTIFKAN LAYAR", mengatur jumlah nomor berikutnya dan dilewati, lalu membuka tautan `/display/<token>` di layar (tanpa login). Satu kartu per counter, berubah langsung saat Next/Skip dan saat counter ditambah atau dihapus. "GANTI TAUTAN" mematikan tautan lama.
- Mode terang dan gelap.

Yang belum tersedia (lihat [DESIGN.md](docs/DESIGN.md#7-batasan-dan-yang-belum-dibangun)): email,
pembatalan oleh customer, hapus nomor oleh owner, counter kustom, catatan, ekspor riwayat, login Google,
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
docker-compose.yml  Stack development (hot reload): db, qr-service, notify-service, mail-service, mailpit, backend, frontend; hanya nginx yang membuka port (Mailpit: 127.0.0.1:1025 dan :8025)
docker-compose.prod.yml  Stack demo/produksi (target prod, nginx hanya di 127.0.0.1)
notify-service/   Web Push (LISTEN Postgres), /health :8091, log JSON
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
| notify-service | `notify-service/Dockerfile` | Target `dev`, hanya di jaringan internal. LISTEN ke Postgres (role `notify_ro`), kirim Web Push (VAPID). Tidak bergantung pada email |
| mail-service | `mail-service/Dockerfile` | Target `dev`, hanya di jaringan internal (:8092). Satu-satunya pengirim email: API `POST /internal/v1/send` (`X-Service-Token`), templat, SMTP, coba ulang |
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
