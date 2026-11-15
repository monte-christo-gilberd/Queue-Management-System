# Ringkasan Arsitektur — Queue Management System

Diperbarui 10 November 2026 agar sesuai kode: enam service (backend, auth, qr, notify, mail, payment), outbox email, layar publik, pembayaran, dan verifikasi email. Login dipisah ke `auth-service` (fase 1) dan ditambah login Google (fase 2).

## Gambaran besar

```
   HP / browser
        │  HTTP (LAN) atau HTTPS (Cloudflare tunnel)
        ▼
 ┌──────────────┐   satu-satunya port yang terbuka ke host (8080; prod hanya 127.0.0.1)
 │    nginx     │   rate limit login, /join, reset password, avatar, pembayaran; SSE tanpa buffer
 └──┬────┬────┬─┘
    │    │    │ /internal/ → selalu 404
    │    │    └──────────────► frontend (Expo web; Metro di dev, statis di prod)
    │    ├── /api/ ──────────► backend (Go, Gin) ──► PostgreSQL
    │    │                         ├──► qr-service (token join, internal saja)
    │    │                         └──► mail-service (email reset password dan verifikasi, internal saja)
    │    ├── /api/v1/payments/ ► payment-service ──► PostgreSQL (role payment_rw) + Stripe
    │    └── login, register, reset password, verifikasi email ► auth-service ──► PostgreSQL (role auth_rw)
    │                              (menerbitkan JWT RS256; backend dan payment memverifikasi lokal, tanpa memanggilnya)

 PostgreSQL ─ pg_notify('queue_events') ─► notify-service ─► Web Push
 PostgreSQL ─ notification_outbox       ─► mail-service   ─► SMTP (Mailpit di dev)
    jaringan Docker internal: db, backend, qr, notify, mail, payment, frontend tidak membuka port
```

Enam service Go, satu database, satu frontend, satu proxy (dev juga menjalankan Mailpit). Hanya nginx yang terhubung ke luar. Service pendukung tidak bisa dijangkau dari luar, kecuali `payment-service` (hanya `/api/v1/payments/`) dan `auth-service` (hanya path `/api/v1/users/...` untuk login, register, reset password, dan verifikasi email). `payment-service` dan `auth-service` terpisah dari antrian: backend tidak memanggil keduanya dan sebaliknya (dijaga `scripts/check-service-isolation.sh`).

## 1. Database (PostgreSQL)

16 tabel: `users`, `owners`, `payments`, `queues`, `queue_settings`, `counters`, `tickets`, `ticket_notes`, `queue_history`, `push_subscriptions`, `user_avatars`, `notification_outbox`, `user_action_tokens`, `queue_displays`, ditambah `payment_webhook_events` dan `payment_session_credits` (dibuat skrip `05_payment_role.sh`, milik payment-service).

| Keputusan | Alasan |
|---|---|
| `sort_key` gap-based, bukan posisi sekuensial | Requeue cukup menulis satu baris |
| Posisi customer dihitung, tidak disimpan | Menghindari sinkronisasi manual dan race condition |
| Sort key tiket baru = max semua tiket + 65536 | `UNIQUE (queue_id, sort_key)` juga mencakup tiket selesai. Menghitung dari tiket `waiting` saja bertabrakan setelah antrean habis (bug 503 minggu 14) |
| Customer wajib login (`tickets.user_id`) | Anti join ganda berbasis akun |
| Satu counter default otomatis per antrean (`is_default`), CRUD counter kustom (maks 10 per antrean) | Menyederhanakan model setelah admin dihapus. Satu akun bisa dipakai di beberapa perangkat, tiap perangkat satu counter |
| `queues.ticket_prefix` (1-3 huruf/angka, default `A`) | Hanya tampilan: label tiket `A2`. `sort_key` dan nomor tiket tidak berubah. Dikembalikan juga di `/users/me/tickets` |
| Tidak ada penghapusan permanen, tanpa batas riwayat, tanpa batas harian QR, tanpa `registration_limit` | Keputusan produk 31 Okt |
| Foto profil di tabel `user_avatars` (BYTEA, maks 150 KB, privat) | Ikut backup database; jalur pindah ke object storage lewat `service.AvatarStore` |
| Token reset password dan verifikasi email di `user_action_tokens` (hanya hash SHA-256) | Satu tabel untuk `password_reset`, `email_verify`, `email_change` (yang terakhir belum dipakai); `users.password_changed_at` mencabut sesi lama |
| Email notifikasi lewat `notification_outbox` (trigger, satu transaksi dengan perubahan tiket) | Email tidak hilang bila SMTP atau mail-service mati |
| Layar publik: `queue_displays` dengan token acak di URL | Tanpa login; `notify_ro` dan `mail_rw` tidak punya hak di tabel ini |
| `owners.subscription_expires_at`, bukan boolean | "Aktif" dihitung, tanpa cron |
| `ON DELETE CASCADE` di FK anak `queues` | Hapus antrean tidak macet |

**Dihapus Oktober 2026:** `queue_authz`, `queue_admins`, `owner_saved_admins`. Sistem admin dibuang seluruhnya. Hanya owner yang mengelola antrean.

Indeks tambahan ada di `02_Indexes.sql` (daftar antrean owner, riwayat tiket customer, tiket selesai). Indeks `users.username` ditunda. Detail tabel: `Database_Schema_Documentation.md`.

## 2. Backend (Go + Gin)

Layering `handler → service → repository`, dependency injection manual di `cmd/server/main.go`. Interface kecil didefinisikan di layer yang memakainya, sehingga service bisa dites dengan fake tanpa database.

```
internal/
  apperror/ config/ database/ dto/ handler/ middleware/ model/
  export/      ekspor riwayat ke CSV dan XLSX
  httpserver/  imageproc/ (foto profil)
  mailclient/  klien HTTP ke mail-service
  qrclient/    klien HTTP ke qr-service (token join)
  repository/  service/  router/  sse/  validator/
  test/        dto, handler, integration, qr, repository, router, service, sse
```

| Fitur | Status |
|---|---|
| Profil, preferensi, foto profil. Register dan login pindah ke `auth-service` (2e); backend hanya memverifikasi JWT RS256 secara lokal | Selesai |
| Lupa dan reset password lewat email (token sekali pakai, sesi lama dicabut) | Selesai |
| Verifikasi email (`POST /users/verify-email`, `POST /users/me/verify-email/resend`, guard di buat antrean dan join, `EMAIL_NOT_VERIFIED` 403) | Selesai: halaman publik `/verify-email`, banner dan tombol kirim ulang di Profil. Dev lewat Mailpit; prod lewat `MAIL_MODE` |
| Buat, lihat, ubah, tutup antrean (butuh langganan aktif untuk membuat) | Selesai |
| Join lewat token QR bertanda tangan | Selesai |
| Join dua kali oleh user yang sama | 200 dengan tiket yang sama |
| Next (panggil tiket waiting pertama, tandai `completed`) | Selesai |
| QR statis dan dinamis (TTL 120 detik), ganti kode statis | Selesai |
| SSE per antrean (`ticket_joined`, `ticket_next`, `ticket_skipped`, `ticket_cancelled`, `ticket_removed`, `counter_updated`, `queue_closed`, `queue_updated`, `queue_qr_updated`) | Selesai |
| Akses antrean orang lain | 404, bukan 403 |
| Skip (hapus, kembali ke belakang, atau setelah N tiket) dan panggil ulang | Selesai |
| Customer membatalkan tiket (`/cancel`), owner menghapus nomor (`/tickets/:id/remove`) | Selesai |
| Sisip customer di posisi tertentu (`/insert`) | Selesai |
| Catatan per nomor (`GET/PUT /tickets/:id/note`) | Selesai |
| Counter kustom (`/queues/:id/counters`) | Selesai |
| Layar publik tanpa login (`/display/:token`, room SSE terpisah) | Selesai |
| Riwayat owner dan customer, ekspor CSV/XLSX (`/history/*`, `/users/me/history`) | Selesai |
| Notifikasi Web Push (`notify-service`) dan email (`mail-service`, outbox) | Selesai |
| Prefix tiket kustom | Selesai |
| Pembayaran langganan (`payment-service`, Stripe) | Selesai (service terpisah) |
| Shutdown rapi (SIGINT/SIGTERM, `SHUTDOWN_TIMEOUT`) | Selesai |

`TRUSTED_PROXIES=172.28.0.0/16` agar IP klien dari `X-Forwarded-For` hanya dipercaya bila datang dari nginx.

## 2b. notify-service (Go, terpisah)

Mendengarkan `pg_notify('queue_events')` dari trigger `tickets_notify_trg`, membaca data lewat role `notify_ro`, mengirim Web Push (VAPID). Hanya port internal 8091 untuk `GET /health` (200 bila LISTEN aktif; dipakai healthcheck compose, tidak dipublikasikan nginx). Tanpa outbox (push saat service mati hilang; email sudah memakai outbox, lihat 2c). Pengiriman memakai timeout 8 dtk dan coba ulang 3 kali untuk kegagalan sementara; log JSON; shutdown menunggu push yang sedang berjalan. Backend hanya menyediakan `recall` dan endpoint `/push/*`.

## 2c. mail-service (Go, terpisah)

Satu-satunya pengirim email; port internal 8092, tanpa rute nginx. Dua jalur: (1) API `POST /internal/v1/send` (`X-Service-Token`) yang dipakai backend untuk email reset password dan verifikasi email, dan (2) worker outbox: trigger `tickets_email_outbox` menulis `notification_outbox` dalam transaksi yang sama dengan perubahan tiket (masuk, dibatalkan, dihapus owner, selesai), lalu worker mengirim lewat SMTP dengan coba ulang berlipat, rate limit, dan `FOR UPDATE SKIP LOCKED` (at-least-once). Enam templat (`joined`, `cancelled`, `removed`, `completed`, `password-reset`, `email-verify`). Role database `mail_rw` dengan hak minimal. Dev memakai Mailpit; prod memilih `MAIL_MODE=smtp|mailpit`. Notify-service tidak bergantung padanya.

## 2e. auth-service (Go, terpisah)

Login, registrasi, login dengan Google, lupa/reset password, dan verifikasi email (`POST /api/v1/users/{register,login,google,google/link,forgot-password,reset-password,verify-email}` dan `POST /api/v1/users/me/verify-email/resend`). Path dan bentuk respons sama dengan yang dulu dilayani backend, jadi frontend tidak berubah; nginx mengarahkan path itu ke port internal 8094. Satu-satunya penerbit token: JWT **RS256** (klaim `sub`, `iat`, `exp` 24 jam, header `kid`) ditandatangani kunci privat `keys/auth-private.pem` yang hanya di-mount ke service ini (dibuat `scripts/gen-auth-keys.sh`). Role database `auth_rw` hanya boleh membaca dan menulis kolom login di `users` serta tabel `user_action_tokens` (tanpa `DELETE`, tanpa akses antrian atau tiket). Email dikirim lewat mail-service (HTTP, tanpa `depends_on`).

**Terisolasi dari antrian.** Backend dan payment-service memverifikasi token **secara lokal** dengan kunci publik `keys/auth-public.pem`, dan pencabutan sesi (`users.password_changed_at` lebih baru dari `iat`) dibaca langsung dari database. Guard verifikasi email di join dan buat antrian juga membaca `users.email_verified_at` dari database. Karena itu, bila auth-service mati: user yang sudah login tetap bisa memakai antrian, tiket, SSE, profil, dan pembayaran; login baru, registrasi, reset password, dan verifikasi email dijawab nginx dengan JSON 503 `AUTH_UNAVAILABLE` (header `Retry-After: 30`) dan frontend **tidak** logout (hanya 401 yang mengakhiri sesi). Terbukti oleh tes chaos (lihat bagian 6). Verifikasi menegakkan algoritma per kunci (RS256 hanya dengan kunci publik), menolak `alg=none`, algorithm confusion, dan token ber-klaim `purpose` (link token Google bukan sesi). **Login Google** (opsional, `GOOGLE_CLIENT_IDS`): ID token diperiksa di server dengan JWKS Google; email yang sudah terdaftar dengan password tidak pernah ditautkan otomatis, pemiliknya harus membuktikan password (link token 5 menit, sekali pakai). `JWT_LEGACY_HS256_SECRET` opsional hanya untuk masa transisi 24 jam agar token HS256 lama tetap sah; kosongkan setelahnya. Dijaga `scripts/check-service-isolation.sh`.

## 2d. payment-service (Go, terpisah)

Satu kemampuan: perpanjang langganan owner 1 bulan setelah Stripe mengonfirmasi pembayaran (`POST /api/v1/payments/checkout`, `POST /api/v1/payments/webhooks/stripe`, `GET /health`, port internal 8093). Pembayaran pertama user biasa sekaligus menjadikannya owner. Webhook idempoten per event dan per sesi Checkout. Role database `payment_rw`; bila service mati nginx menjawab 503 `PAYMENT_UNAVAILABLE` dan antrian tetap jalan. Di dev tersedia simulator (`DEV_PAYMENT_SIMULATOR`), ditolak di produksi.

## 3. qr-service (Go, terpisah)

Stateless, tanpa database. Membuat dan memverifikasi token join bertanda tangan HMAC (`JOIN_LINK_SECRET`), dengan jenis statis atau dinamis (TTL). Hanya backend yang memanggilnya lewat `/internal/v1/*` dengan `X-Service-Token`. Gambar QR **tidak** dibuat di sini lagi: endpoint publik `/qr/join.png`, `image_url`, dan `PUBLIC_QR_URL` sudah dihapus. Backend hanya mengembalikan `join_url`.

Kode error QR: `QR_INVALID` 404, `QR_EXPIRED` 410, `QR_UNAVAILABLE` 503, `QUEUE_CLOSED` 409.

## 4. Frontend (Expo / React Native Web)

Expo Router, NativeWind, Zustand. Satu basis kode untuk web dan native.

- **Rute**: `login`, `register`, `forgot-password`, `reset-password`, `join`, `ticket/[queueId]`, `display/[token]` (publik), dan `(owner)/`: dashboard, antrian, detail dan `live` per antrean, riwayat, profil (edit), `subscription`.
- **QR digambar di frontend** dari `join_url` memakai `qrcode` dan `react-native-svg`. Level koreksi galat `L` dan ukuran dibulatkan ke kelipatan piksel agar mudah dipindai kamera HP.
- **Join hanya lewat pemindai kamera dalam aplikasi** (`expo-camera`), yang langsung memanggil `POST /join {t}`. Halaman `/join?t=` hanya menampilkan info.
- **Real-time lewat SSE dengan `fetch`**, bukan `EventSource`, karena stream butuh header `Authorization`.
  - Reconnect dengan backoff 1 detik sampai 30 detik.
  - Resinkron saat tab aktif lagi dan tiap 45 detik.
  - Bila stream tidak mengirim satu byte pun selama 3 detik (terjadi lewat Cloudflare quick tunnel yang menahan stream), data diambil ulang tiap 3 detik.
- Kamera di browser butuh HTTPS atau `localhost`.

## 5. Deploy dan development

`docker-compose.yml` untuk development dan `docker-compose.prod.yml` untuk demo/produksi (project `qms-prod`, subnet `172.29.0.0/16`, env dari `.env.prod`, frontend statis, `service_healthy` untuk backend dan qr-service). Keduanya tidak bisa jalan bersamaan (port 8080). Di prod, HTTPS lewat Cloudflare Tunnel `qms-ta` ke `https://qms.montechristo.my.id` (service `cloudflared` di host, dinyalakan manual), akses dibatasi aturan WAF berdasarkan IP WiFi, dan header `Upgrade` Metro tidak dikirim. Skema DB yang sudah berjalan diperbarui dengan `db/scripts/apply_schema.sql` (idempoten). Kelima service Go memakai Go 1.27. Dockerfile multi-stage (`dev` dan `prod`). Go memakai `go run` dengan restart manual (`docker compose restart backend`), frontend memakai Metro dengan hot reload.

nginx mengikuti `template-nginx/`: `deploy/nginx/nginx.conf` dan `server.conf`. Rute:

| Path | Tujuan | Catatan |
|---|---|---|
| `/api/v1/queues/:id/event` | backend | SSE, tanpa buffer, timeout 86400 detik |
| `/api/v1/users/login`, `/register` | backend | Rate limit 10 r/s per IP |
| `/api/v1/users/forgot-password`, `/reset-password` | backend | `reset_zone` 1 r/s, burst 5; log halaman `/reset-password` tanpa query |
| `/api/v1/users/me/avatar` | backend | Body maks 6m, `avatar_zone` 5 r/s |
| `/api/v1/display/:token[/event]`, `/display/:token` | backend, frontend | Layar publik: SSE tanpa buffer, token disamarkan di log, tanpa Referer dan cache |
| `/api/v1/payments/` | payment-service | `payment_zone`; webhook Stripe tanpa rate limit; 503 JSON bila service mati |
| `/api/v1/join` | backend | Rate limit 10 r/s per IP |
| `/api/` | backend | |
| `/internal/` | | Selalu 404 |
| `/` | frontend | Termasuk WebSocket Metro untuk hot reload |

Akses dari HP, lewat `Makefile`:

| Perintah | Hasil | Kamera HP |
|---|---|---|
| `make up` | `http://<IP LAN>:8080` | Tidak jalan (bukan HTTPS) |
| `make tunnel` | `https://*.trycloudflare.com` | Jalan |
| `make down` | Hentikan | |

mkcert dan listener HTTPS lokal sudah dibuang; HTTPS hanya lewat tunnel. Perintah harian ada di `Makefile` (`up`, `tunnel`, `down`, `restart`, `seed`).

## 6. Pengujian

| Lapisan | Alat | Hasil terakhir |
|---|---|---|
| Backend unit | Go test, fake manual | Lulus |
| Backend integrasi | Go test dengan tag `integration`, skema sementara di Postgres | Butuh `DATABASE_URL` |
| qr-service | Go test hitam-putih di `internal/test/*_test/` | Lulus |
| Frontend unit | Runner bawaan Node | Lulus (sekitar 159 tes) |
| mail-service, notify-service, payment-service | Go test (payment: integrasi DB bila env diisi; verifikasi token RS256, secret lama, dan algorithm confusion) | Lulus |
| auth-service | Go test (token RS256, registrasi, login, reset password, verifikasi email, pencabutan sesi, handler, konfigurasi, SQL lewat sqlmock) | Lulus |
| Tes chaos auth-service | Stack sungguhan: `docker compose stop auth-service`, lalu user yang sudah login memakai `/users/me`, buat antrian, Next, join, SSE, dan checkout dengan token yang sama; login baru, register, dan reset password menjawab 503 `AUTH_UNAVAILABLE`; HS256 palsu ditolak; setelah dinyalakan lagi login pulih | 38 dari 38 lulus (10 Nov 2026) |
| E2E | Playwright, sekitar 217 tes di 32 spec, seed `db/scripts/reset_seed_dev.sql` | Tes email butuh Mailpit; dua tes ketahanan butuh `QMS_DOCKER_EXEC=1` |

## 7. Belum dibangun

Rencana sebelumnya (counter, riwayat dan ekspor, papan publik, pembatalan, hapus nomor, pembayaran, lupa password, email) sudah selesai. Tersisa:

- Login Google: tombol di layar masuk dan daftar hanya tampilan; kolom `users.google_id` ada, backend belum punya OAuth.
- Ganti email (`email_change`) belum dibangun; kolom `purpose` sudah menyiapkannya.
- Observability (metrik, log terpusat, dashboard, alert) dan uji beban k6: belum ada.
- Outbox untuk Web Push, migration runner, backup otomatis, HSTS dan Cloudflare Access, backend lebih dari satu instance (hub SSE di memori).
- QR join di layar publik (kolom `show_qr` belum ada). Plan langganan selain 1 bulan.
- Dibatalkan (keputusan 31 Okt): batas harian QR dan `registration_limit`.
- Uji manual yang butuh perangkat: hot reload Metro lewat nginx dan resinkron `AppState` di native.
