# Design — Queue Management System

Dokumen ini menjelaskan arsitektur, keputusan desain, dan batasan sistem. Untuk bentuk request dan response
lihat [api-contract.md](api-contract.md). Untuk alur pengguna lihat [FLOW.md](FLOW.md).

## 1. Gambaran sistem

Sistem antrian untuk layanan tatap muka. Ada dua peran:

- **Owner**: membuat antrian, memanggil nomor (Next, Skip, Panggil ulang), mengelola counter, menyisipkan atau menghapus nomor, menulis catatan, mengaktifkan layar publik, melihat riwayat dan mengekspornya, dan menutup antrian. Owner harus punya langganan aktif (diperpanjang lewat `payment-service`). User biasa menjadi owner lewat pembayaran pertama.
- **Customer**: login, bergabung ke antrian lewat token QR atau tautan, melihat nomor yang sedang dilayani dan posisinya secara real-time, membatalkan tiketnya, dan melihat riwayat tiket. Menerima Web Push dan email.

Sistem terdiri dari lima service Go (backend antrian, `qr-service`, `notify-service`, `mail-service`, `payment-service`), satu PostgreSQL, satu frontend Expo, dan nginx. Ringkasan lintas-service ada di `Architecture_Summary.md`.

```
   Browser (Expo web / native)
        │  HTTP (REST, JSON)  +  SSE (fetch stream)
        ▼
   nginx  ── frontend (Metro di dev, statis di prod)
        │  /api/  ──────────────►  Backend Go (Gin) :8080 ──► PostgreSQL (qms_db)
        │                              ├──► qr-service :8090   (token join)
        │                              └──► mail-service :8092 (email reset password, verifikasi)
        │  /api/v1/payments/ ───►  payment-service :8093 ──► PostgreSQL (payment_rw) + Stripe

   PostgreSQL ─ pg_notify('queue_events') ─► notify-service :8091 ─► Web Push
   PostgreSQL ─ notification_outbox       ─► mail-service         ─► SMTP
```

Dalam produksi nginx berada di depan (lihat `deploy/nginx/`). Frontend dan API berada di satu origin,
sehingga CORS hanya dibutuhkan saat frontend dibuka dari origin lain.

## 2. Backend

### 2.1 Lapisan

```
handler  →  service  →  repository  →  PostgreSQL (sqlx + pgx)
   │           │
   │           └── notifier (SSE hub)
   └── dto (bentuk JSON)
```

- **handler** (`internal/handler`): membaca request, memanggil service, dan memetakan error ke kode HTTP dan kode error.
  Tidak berisi aturan bisnis.
- **service** (`internal/service`): semua aturan bisnis (kepemilikan, langganan, urutan antrian, join ulang).
- **repository** (`internal/repository`): SQL. Tidak tahu aturan bisnis.
- **dto** (`internal/dto`): bentuk JSON ke klien. Model database tidak pernah dikirim langsung.

Setiap lapisan mendefinisikan interface yang dibutuhkannya sendiri (mis. `TicketRepository` di paket service).
Akibatnya service bisa diuji dengan fake repository tanpa database, dan wiring dilakukan manual di
`cmd/server/main.go`.

### 2.2 Autentikasi dan otorisasi

- Password di-hash dengan bcrypt (cost 10). Respons tidak pernah memuat password.
- Token JWT HS256 berlaku 24 jam. Klaim `sub` berisi user id, `iat` waktu terbit. Middleware
  `RequireAuthWithRevocation` memeriksa token di semua rute `/api/v1/queues` dan `/api/v1/users/me`, dan menolak token
  ber-`iat` yang lebih lama dari `users.password_changed_at` (sesi dicabut setelah reset password; perbandingan dalam
  detik). Token tanpa `iat` (terbitan lama) tetap sah sampai `exp` dan tidak memicu query. Biayanya satu query per
  request terautentikasi, diukur di item 13.
- Password minimal 8 karakter dan maksimal 72 byte (`service.ValidatePassword`, dipakai registrasi dan reset).
- **Reset password (item 9):** `POST /users/forgot-password` selalu 202 (tanpa enumerasi; pengiriman di goroutine,
  maksimal 32 bersamaan, selebihnya 503). Token 32 byte acak, hanya hash SHA-256-nya di `user_action_tokens`
  (30 menit, sekali pakai, token baru membatalkan yang lama, maksimal 3 per akun per jam). Backend memanggil
  `mail-service` (`POST /internal/v1/send`, templat `password-reset`) lewat `internal/mailclient`; `MAIL_SERVICE_URL` dan
  `MAIL_SERVICE_TOKEN` diisi berpasangan, kosong = rute tidak dipasang. `POST /users/reset-password` mengonsumsi token,
  mengganti password, mengisi `password_changed_at`, dan membatalkan token lain dalam satu transaksi; password
  diperiksa sebelum token disentuh. nginx: zona `reset_zone` (1 r/s, burst 5), log halaman `/reset-password` memakai
  format tanpa query (`qms_masked`) supaya token tidak tercatat, dan `Referrer-Policy: no-referrer`. `notify_ro` dan
  `mail_rw` tidak punya hak apa pun di `user_action_tokens`. Tabel yang sama disiapkan untuk verifikasi ganti email
  (`purpose = 'email_change'`), yang belum dibangun.
- Login menerima email atau username. Pesan gagal login tidak membedakan mana yang salah.
- **Verifikasi email:** `EMAIL_VERIFICATION_REQUIRED` (bawaan: menyala bila `MAIL_SERVICE_URL` terisi; menyalakannya tanpa mail-service membuat backend menolak start). Registrasi mengirim tautan (`email-verify`, `/verify-email?token=`, berlaku 24 jam, maks 3 per jam per akun) memakai `user_action_tokens` (`purpose = 'email_verify'`) dan kolom `users.email_verified_at`. `POST /users/verify-email {token}` (publik; token tidak sah, kedaluwarsa, atau terpakai: 400 `INVALID_TOKEN`), `POST /users/me/verify-email/resend` (202 netral, 429 bila terlalu sering). `GET /users/me` memuat `email_verified` (selalu `true` bila fitur mati). Guard `RequireVerified` membaca database pada tiap panggilan (bukan klaim JWT) dan menolak 403 `EMAIL_NOT_VERIFIED` untuk buat antrian dan join nomor baru (join ulang yang mengembalikan tiket lama tidak ditolak). Frontend: halaman publik `/verify-email` dan banner dengan tombol kirim ulang di Profil. Email verifikasi dikirim sinkron-lepas: `Register` mengirimnya di goroutine (gagal hanya masuk log, registrasi tetap 200). Dev: email masuk Mailpit. Prod: ikut `MAIL_MODE` (`smtp` bawaan, mis. Gmail, atau `mailpit`; `scripts/check-mail-mode.sh` memperingatkan bahwa pengguna baru tidak bisa memverifikasi di mode `mailpit`, dan `make verify-user` menandai satu akun). Migrasi DB lama (`db/scripts/apply_schema.sql`) menambah kolom, memperluas CHECK `purpose`, dan memverifikasi semua akun yang sudah ada sekali saja (kolom belum ada), supaya tidak ada yang terkunci. Seed dan akun demo dibuat sudah terverifikasi. Tidak ada cek MX atau allowlist domain: bukti kepemilikan email adalah klik tautan.
- Foto profil: lihat 6c.

Aturan akses:

| Operasi | Siapa boleh |
|---|---|
| Buat, ubah, atur izin, daftar antrian | Owner dengan langganan aktif (daftar tetap ada tanpa langganan, tetapi hanya antrian aktif) |
| Tutup, detail, daftar tiket, Next, Skip, Panggil ulang, sisip, hapus nomor, counter, catatan, pengaturan layar publik | Owner antrian |
| Batalkan tiket | Pemegang tiket |
| Riwayat antrian dan ekspor (`/history/*`) | Owner antrian; customer memakai `/users/me/history` |
| Layar publik (`/display/:token`) | Siapa pun yang memegang token (tanpa login) |
| Info antrian (`/info`) | Semua user yang login |
| Status antrian dan SSE | Owner atau pemegang tiket di antrian itu |
| Join | Semua user yang login, selama antrian aktif |

Permintaan yang tidak diizinkan dibalas **404**, bukan 403. Dengan begitu keberadaan antrian milik orang lain
tidak bisa ditebak.

### 2.3 Model data

Semua tabel di skema kini dipakai (16 tabel; detail di `Database_Schema_Documentation.md`):

| Tabel | Fungsi |
|---|---|
| `users` | Akun (email, username, password hash, `email_notifications`, `password_changed_at`, `email_verified_at`) |
| `owners` | Penanda owner dan masa berlaku langganan (`subscription_expires_at`) |
| `payments` | Catatan tiap pembayaran (ditulis `payment-service`) |
| `queues`, `queue_settings` | Antrian (nama, deskripsi, status, pemilik, `ticket_prefix`) dan pengaturan Skip (`skip_fallback`, `skip_fallback_n`, `skip_max_returns`) |
| `counters` | Counter per antrian (satu default, maks 10) |
| `tickets`, `ticket_notes`, `queue_history` | Tiket (`waiting`, `currently_served`, `completed`, `cancelled`, `skipped`, `deleted`), catatan owner, dan riwayat |
| `push_subscriptions` | Langganan Web Push per browser |
| `notification_outbox` | Antrian email yang dikirim mail-service |
| `user_action_tokens` | Hash token reset password dan verifikasi email |
| `user_avatars` | Foto profil |
| `queue_displays` | Token dan pengaturan layar publik |
| `payment_webhook_events`, `payment_session_credits` | Milik payment-service (idempotensi webhook) |

Batasan penting di `tickets`:
- `UNIQUE (queue_id, user_id)`: satu user hanya punya satu tiket per antrian, termasuk setelah tiketnya selesai.
- `UNIQUE (queue_id, sort_key)`: dua tiket tidak boleh punya urutan yang sama, termasuk yang sudah selesai.

### 2.4 Urutan antrian (sort_key)

- Setiap tiket baru mendapat `sort_key` = nilai maksimum `sort_key` di antrian itu + **65536** (gap).
- Maksimum dihitung dari **semua** tiket di antrian, bukan hanya yang `waiting`. Alasannya: `UNIQUE (queue_id, sort_key)`
  berlaku untuk tiket yang sudah selesai juga. Kalau hanya tiket `waiting` yang dihitung, antrian yang sudah habis akan
  menghasilkan kunci yang bentrok dan join gagal terus (ini bug yang pernah terjadi).
- Gap besar memungkinkan penyisipan di tengah (`POST /insert`) tanpa menulis ulang banyak baris.
- Next memilih tiket `waiting` dengan `sort_key` terkecil.

### 2.5 Siklus tiket

```
            join                 next
  (baru) ───────────► waiting ───────────► currently_served ───────► completed
                         │  ▲                     │      (Next berikutnya / antrian ditutup)
                         │  └── skip dengan fallback (back | after_n) atau counter dihapus
                         ├── cancel (customer)            ──► cancelled
                         ├── remove (owner)               ──► deleted
                         └── skip permanen (fallback `remove`) ──► skipped
```

Aturan Skip diatur per antrian lewat `queue_settings.skip_fallback` (`remove|back|after_n`), `skip_fallback_n`, dan `skip_max_returns`. Setiap counter melayani satu nomor (`currently_served`). Menghapus counter yang sedang melayani mengembalikan nomornya ke depan antrian.

- **Join ulang** (user yang sudah punya tiket di antrian itu): backend mengembalikan **tiket yang sudah ada** dengan 200.
  Tidak ada nomor kedua dan tidak ada event baru. Ini menjawab kasus scan QR dua kali atau double-tap.
- **Next** pada antrian tanpa tiket `waiting`: 409 `QUEUE_EMPTY`, tanpa perubahan data.

### 2.6 Nomor yang ditampilkan

- `current_number` = `ticket_number` tiket `currently_served` (satu per counter; customer melihat nomor yang dilayani di antrian itu).
- `previous_number` = tiket yang baru selesai (`completed`) sebelumnya.
- `next_number` = tiket `waiting` pertama.
- `waiting_ahead` = jumlah tiket `waiting` dengan `sort_key` lebih kecil dari milik requester.

Tampilan customer memakai angka ini langsung dari `GET /status`. Papan owner berbeda: ia memuat daftar tiket
(`GET /tickets`) lalu menurunkan posisi yang sama di frontend (`utils/queue-view.ts`), dengan aturan yang sama dengan di atas.

### 2.7 Real-time (SSE)

- Setiap antrian punya satu *hub* di memori. Klien yang tersambung ke `/queues/:id/event` didaftarkan ke hub itu.
- Service memanggil `notifier` setelah transaksi selesai (join, next, skip, cancel, remove, counter, update, close, QR). Event: `ticket_joined`, `ticket_next`, `ticket_skipped`, `ticket_cancelled`, `ticket_removed`, `counter_updated`, `queue_closed`, `queue_updated`, `queue_qr_updated`, dan `display_changed` (hanya ke room layar publik, lihat 6d). Hub mengirim event ke semua klien di antrian itu.
- Setiap ~25 detik dikirim heartbeat `: ping` agar koneksi tidak diputus proxy. Header `X-Accel-Buffering: no` mematikan buffering di nginx.
- Klien yang putus dibersihkan dari hub (`defer Unsubscribe`).
- Event hanya membawa data yang perlu untuk tampilan. `user_id` dan `sort_key` dibuang dari `TicketEvent`, karena penerima bisa
  customer lain di antrian yang sama.

**Batasan**: hub ada di memori satu proses. Menjalankan lebih dari satu instance backend memerlukan mekanisme pub/sub bersama
(mis. PostgreSQL LISTEN/NOTIFY atau Redis) supaya event sampai ke semua instance.

### 2.8 Format respons

- Snake_case dan `null` untuk field kosong, dibentuk lewat DTO (`NewXxxResponse`).
- Sukses dibungkus `{"data": ...}`, gagal dibungkus `{"error": {"code", "message"}}`.
- Kode error adalah string tetap (`NOT_FOUND`, `QUEUE_EMPTY`, dst.) agar klien bisa menangani tanpa membaca teks pesan.

## 3. Frontend

### 3.1 Teknologi

- **Expo SDK 57** dengan Expo Router (routing berbasis file). Target utama web (`output: static`), native menyusul.
- **NativeWind** (Tailwind untuk React Native) untuk gaya.
- **Zustand** dengan `persist` untuk sesi login (token dan nama tampilan). Disimpan di AsyncStorage, di web sebagai localStorage.
- **SSE** lewat `fetch` stream (bukan `EventSource`), karena `EventSource` tidak bisa mengirim header `Authorization`.

### 3.2 Struktur

```
src/
  app/          rute (Expo Router). Membaca store, memanggil hook/lib, meneruskan data ke komponen.
  components/   komponen presentational: hanya props, tanpa fetch dan tanpa store.
  hooks/        data dari server (use-queue-board, use-queue-spot, use-my-tickets, use-queue-events, ...).
  lib/          klien HTTP (api.ts), fungsi per endpoint (tickets.ts, queues.ts), util murni.
  store/        authStore (sesi login).
  utils/        logika murni (queue-view.ts: nomor sebelumnya/sekarang/berikutnya).
  constants/    tokens.js: satu sumber warna dan radius.
```

Aturan: komponen di `components/` tidak mengimpor store dan tidak memanggil API. Dengan begitu tampilan bisa diuji
dan dirombak tanpa menyentuh data.

### 3.3 Routing dan penjagaan

- `(owner)/` dijaga `AuthGate` mode `required`. Tanpa token, user diarahkan ke login dengan parameter `next`.
- `join` (`/join?t=<token>`) juga memakai `required`: user harus login sebelum bergabung. `AuthGate`
  menerima `nextPath` eksplisit agar query token tidak hilang saat diarahkan ke login.
- `display/[token]` publik (tanpa `AuthGate`): layar untuk monitor ruang tunggu.
- `forgot-password`, `reset-password`, dan `verify-email` publik; `(owner)/` juga memuat riwayat, profil, `subscription`, dan `queues/[queueId]/live`.
- `ticket/[queueId]` menampilkan status tiket yang sudah dimiliki (dibuka dari Antrian Saya/Riwayat).
- Login dan register memakai `guest`: user yang sudah login langsung dibawa ke `next` atau dashboard.
- `next` diperiksa dengan `sanitizeNext` agar tidak bisa dipakai untuk membuka situs lain (open redirect).
- Respons 401 dari API menghapus sesi lokal dan mengarahkan ke login.

### 3.4 Layout responsif

Satu komponen kerangka (`ResponsiveScreenLayout`) dipakai untuk dua tampilan:
- Lebar ≥ 1024 px: header dengan panel profil, konten dibatasi lebar maksimum, dan halaman detail antrian memakai
  layout dua kolom yang tidak di-scroll sebagai halaman (hanya kolom kanan yang di-scroll).
- Lebih sempit: header HP dengan tombol kembali, tab bar bawah, dan panel profil dari kanan.

Tab-root (Beranda, Antrian Saya, Riwayat, Profil) tidak punya tombol kembali. Halaman turunan punya.

### 3.5 Tema

- Warna semantik (`background`, `surface`, `foreground`, `line`, dst.) didefinisikan sekali di `tokens.js` untuk mode terang dan gelap.
- Mode diatur lewat `colorScheme` NativeWind. Pilihan pengguna disimpan dengan kunci `qms-theme`.
- Warna tetap (mis. papan antrian dan header gelap) tidak ikut tema, karena memang dirancang gelap di kedua mode.
- Warna yang diberikan sebagai prop non-style (ikon, spinner) memakai `useThemeColor`.

### 3.6 Konfigurasi alamat API

`EXPO_PUBLIC_API_URL` dipakai saat build. Kalau kosong, frontend memakai host halaman yang dibuka dengan port 8080.
Dengan nginx, `EXPO_PUBLIC_API_URL` harus diisi dengan origin nginx. Kalau tidak, frontend memanggil port 8080 langsung dan melewati proxy.

## 4. Deployment

- Stack berjalan lewat Docker Compose: `docker-compose.yml` (dev, kode di-mount, hot reload; termasuk Mailpit) dan `docker-compose.prod.yml` (demo/produksi di belakang Cloudflare Tunnel, env dari `.env.prod`, nginx hanya di `127.0.0.1:8080`, frontend statis). Layanan: `db`, `qr-service`, `notify-service`, `mail-service`, `payment-service`, `backend`, `frontend`, `nginx` (dan `mailpit` pada mode tertentu). Keduanya tidak bisa jalan bersamaan (port 8080).
- Hanya nginx yang membuka port. Service internal tidak punya `ports:`; `payment-service` hanya terjangkau lewat `/api/v1/payments/` (503 JSON bila mati) dan sengaja tanpa `depends_on` dari atau ke backend.
- nginx meneruskan `/api/` ke backend, memakai rute SSE tanpa buffer (antrian dan layar publik), rate limit (login, join, reset password, avatar, pembayaran), menyamarkan token di log, dan memblokir `/internal/`. Konfigurasinya ada di `deploy/nginx/`. TLS dipegang Cloudflare; nginx hanya HTTP.
- Backend menolak start di `APP_ENV=production` bila rahasia lemah atau origin bukan https. Shutdown rapi: SIGINT/SIGTERM menutup SSE, menuntaskan request (`SHUTDOWN_TIMEOUT`), lalu menutup database.
- Skema DB: `db/migrations/01*.sql` (satu file per domain; peta di `db/migrations/README.md`) hanya jalan pada data directory kosong; DB yang sudah berjalan memakai `db/scripts/apply_schema.sql` (idempoten) dan skrip role `03`-`05`. Perintah harian lewat `Makefile` dan `scripts/` (backup, akun demo, `check-service-isolation.sh`, `check-mail-mode.sh`).
- Semua variabel sensitif diisi saat container dijalankan. Tidak ada rahasia di dalam image.

## 5. Keputusan desain

| Keputusan | Alasan |
|---|---|
| Join ulang mengembalikan tiket yang ada, bukan error | Spek: customer yang scan ulang harus melihat nomornya. Double-tap tetap menghasilkan satu tiket. |
| Hanya nomor (tanpa nama customer) di tampilan publik | Privasi. Nama tidak ada di data tiket. |
| 404 untuk akses yang tidak diizinkan | Keberadaan data orang lain tidak bocor. |
| SSE lewat `fetch` stream | Header `Authorization` tetap dikirim. Token tidak masuk ke URL. |
| Event SSE tanpa `user_id` dan `sort_key` | Penerima bisa customer lain. |
| Customer menerima nomor yang sudah dihitung backend; owner menurunkannya dari daftar tiket | Customer tidak perlu daftar tiket lengkap. Owner memang butuh semua tiket untuk papan, jadi penurunannya di frontend tidak menambah beban. Aturannya sama di kedua sisi. |
| Layar dan komponen dipisah (presentational) | Tampilan bisa diuji dan dirombak tanpa menyentuh data. |
| Daftar antrian tetap ada tanpa langganan, tetapi hanya yang aktif | Owner tetap bisa melihat antrian yang masih berjalan. |
| Gap 65536 untuk `sort_key` | Penyisipan di tengah bisa dilakukan tanpa menulis ulang baris lain. |

## 6. Strategi pengujian

| Lapisan | Alat | Lokasi |
|---|---|---|
| Unit backend | `go test` dengan fake repository, stub service, spy notifier, dan mock | `backend/internal/test/*_test`, `backend/internal/dto_test` |
| Integration backend | `go test -tags integration` pada PostgreSQL (schema sementara) | `backend/internal/test/integration_test` |
| Unit frontend | `node --test` (`npm run test:unit`) untuk logika murni | `frontend/tests/unit` |
| Unit service lain | `go test ./...` di tiap service | `mail-service`, `notify-service`, `payment-service`, `qr-service` |
| End-to-end | Playwright (tidak dijalankan secara default; tes email butuh Mailpit) | `frontend/playwright` |

Ekspektasi tes diturunkan dari product overview (`documentation/docs/product-overview.md`), bukan dari kode.

## 6b. notify-service (Web Push)

Service Go terpisah, internal, tidak dipublikasikan nginx. Hanya membuka `GET :8091/health` (200 bila LISTEN aktif) untuk healthcheck compose; log JSON; push memakai timeout 8 detik dan coba ulang 3 kali untuk kegagalan sementara. Trigger `tickets_notify_trg` (`AFTER UPDATE OF status, called_at, recalled_at, skip_count`) memanggil `pg_notify('queue_events', {kind: called|recalled|skipped|requeued, queue_id, ticket_id})`; `requeued` juga membawa `reason` (`skip` atau `counter_closed`) dan `counter` (nama counter, dibaca saat trigger karena counter yang dihapus sudah hilang saat notify-service membaca), dan dicek lebih dulu daripada `skipped`; NOTIFY dikirim saat COMMIT. `notify-service` LISTEN (harus ke primary), membaca data dengan role `notify_ro` (SELECT `tickets`, `queues`, `counters`, `push_subscriptions`; DELETE `push_subscriptions`), lalu mengirim lewat webpush-go. Penerima: `called` → pemilik nomor dan tiket waiting terdepan; `recalled` → pemilik nomor; `skipped` (skip permanen) → pemilik nomor yang dilewati; `requeued` (nomor kembali ke `waiting` dari skip dengan fallback atau karena counter dihapus) → pemilik nomor, dengan posisi baru yang dihitung dari `sort_key` (`PositionOf`). Langganan 404/410 dihapus. Teks nomor dibuat satu fungsi (`message.Label`). Struktur: `internal/{config,message,event,store,push,notify}`, tes di `internal/test/`. Belum ada outbox: push saat service mati hilang.

Backend: `POST /queues/:id/recall` (`tickets.recalled_at`, 3 detik, sekali per pemanggilan) dan `GET /push/public-key`, `POST /push/subscribe|unsubscribe`. Langganan unik per `endpoint`; subscribe memindahkannya ke user yang login.

### mail-service (email, microservice tersendiri)

Email dikirim oleh **`mail-service`**, bukan notify-service. Pemisahannya disengaja: `notify-service` hanya Web Push, tidak punya kode, env, atau `depends_on` ke email, jadi SMTP atau mail-service yang mati tidak menghentikan push (dibuktikan dengan peristiwa nyata saat mail-service dimatikan; dijaga `scripts/check-service-isolation.sh`).

- **Bentuk:** modul Go sendiri (`mail-service/`, satu dependensi: `pgx` untuk worker outbox), port internal 8092 (worker outbox: lihat bagian "Outbox email" di bawah; tanpa `DATABASE_URL` worker mati dan hanya API kirim yang jalan), tidak ada rute nginx. `GET /health` tidak memeriksa SMTP. `POST /internal/v1/send` dengan header `X-Service-Token` (`MAIL_SERVICE_TOKEN`, minimal 32 karakter; pola `qr-service`): `{"to","template","data"}`. Isi email ditentukan templat di mail-service; pemanggil hanya memberi nama dan data. Jawaban: 200 `{"data":{"status":"sent"}}`, 400 (JSON rusak, field asing, penerima tidak sah, templat tidak ada, data tidak lengkap), 401, 413 (body > 64 KB), 502 `SMTP_FAILED` (detail SMTP tidak dibocorkan), 503 `BUSY` (lebih dari 16 pengiriman bersamaan). Pengiriman sinkron dengan coba ulang di dalam; tidak ada antrian memori, jadi pemanggil yang butuh ketahanan memakai outbox (item 8, di bawah). Alamat penerima tidak dicatat di log.
- **Paket `internal/mailer`:** pesan MIME `multipart/alternative` (teks dan HTML, quoted-printable, subjek UTF-8), satu tenggat per percakapan SMTP (dial 5 detik, total 15 detik, ikut batal bersama `context`), mode TLS `none`/`starttls`/`tls` (`PlainAuth` menolak kata sandi lewat koneksi tanpa TLS ke host selain localhost), coba ulang 3 kali dengan backoff 1 dan 2 detik untuk kegagalan sementara (dial, 4xx); 5xx dan input tidak sah tidak diulang (`ErrPermanent`); `To` dan `Subject` dengan CR atau LF ditolak.
- **Templat** (`internal/mailer/templates`, embed): tiap templat terdiri dari `<nama>.subject.tmpl`, `.txt.tmpl`, `.html.tmpl` plus `layout.html.tmpl` bersama. HTML lewat `html/template` (nilai pengguna otomatis di-escape); `missingkey=error` membuat data yang kurang menjadi galat, bukan "<no value>" yang terkirim; nama templat divalidasi (`^[a-z0-9][a-z0-9-]*$`).
- **Konfigurasi** divalidasi saat start (`config.Load`): `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`/`SMTP_PASSWORD` (berpasangan), `SMTP_TLS`, `MAIL_SERVICE_TOKEN`; konektivitas tidak diperiksa saat start. `mail-service send-test <alamat>` mengirim email uji.
- **Dev:** Mailpit (`axllent/mailpit`, versi dipin): SMTP 1025 dan UI 8025 diikat ke 127.0.0.1 di host (ubah lewat `MAILPIT_SMTP_PORT`, `MAILPIT_UI_PORT`), tidak lewat nginx; mail-service memakai `mailpit:1025` lewat jaringan internal. **Produksi** memilih lewat `MAIL_MODE` (`.env.prod` atau `make prod-up MAIL_MODE=...`): `smtp` (bawaan) memakai SMTP nyata dan mewajibkan `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `MAIL_SERVICE_TOKEN`; `mailpit` menyalakan Mailpit lewat compose profile `mailpit` (UI hanya 127.0.0.1, pesan di volume) dan memaksa `SMTP_*` ke Mailpit, untuk demo atau uji tanpa mengirim email sungguhan. `scripts/check-mail-mode.sh` menolak konfigurasi yang salah sebelum stack berjalan.


### Outbox email (item 8)

Empat email ke pemilik tiket: `joined` (INSERT tiket; join ulang mengembalikan tiket yang sama tanpa INSERT, jadi tidak ganda; sisip owner juga `joined`), `cancelled`, `removed` (status `deleted`), dan `completed` (hanya dari `currently_served`, termasuk saat antrian ditutup).

- **Tulis atomik.** Trigger `tickets_email_outbox` (`AFTER INSERT OR UPDATE OF status`, terpisah dari `tickets_notify` supaya push tidak tersentuh) menulis `notification_outbox` dalam transaksi yang sama dengan perubahan tiket, lalu `pg_notify('mail_outbox')` untuk membangunkan worker. `UNIQUE (kind, ticket_id)` + `ON CONFLICT DO NOTHING` menjamin satu email per kejadian. Payload berupa snapshot (`queue_id`, `queue_name`, `prefix`, `number`, `name`); **tidak pernah memuat catatan owner** dan label nomor diformat di satu tempat (`outbox.Label`, sama dengan notify-service). Ticket dihapus: baris outbox ikut terhapus (`ON DELETE CASCADE`), yang juga mencegah bentrok id setelah `TRUNCATE ... RESTART IDENTITY` pada seed.
- **Sakelar** `users.email_notifications` (default `TRUE`, diubah lewat `PUT /users/me/preferences`) dibaca **saat kejadian** oleh trigger: mematikan sakelar tidak membatalkan email yang sudah antre, tetapi kejadian sesudahnya tidak masuk outbox, dan menyalakannya lagi tidak mengirim yang terlewat.
- **Worker ada di mail-service**, bukan notify-service: notify-service tetap Web Push murni tanpa dependensi ke email (dijaga `scripts/check-service-isolation.sh`). Worker mengklaim lewat satu pernyataan `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED)` yang menaikkan `attempts` dan memajukan `next_attempt_at` sebesar lease (masa sewa, `SendTimeout + 1 menit`); kunci baris tidak dipegang selama SMTP. Pengiriman **at-least-once**: bila proses mati tepat setelah SMTP menerima pesan dan sebelum `sent_at` ditulis, email bisa terkirim dua kali setelah lease habis.
- **Coba ulang:** kegagalan sementara menjadwalkan `next_attempt_at` dengan backoff berlipat (30 detik, 1, 2, ... maksimum 30 menit; `OUTBOX_MAX_ATTEMPTS` bawaan 8). Gagal permanen (`mailer.ErrPermanent`: 5xx, alamat ditolak), payload atau templat rusak, atau percobaan habis menandai `failed_at` dan berhenti; `last_error` menyimpan sebabnya (maks. 500 karakter). Satu baris rusak tidak menahan baris lain. **Rate limit** `OUTBOX_RATE` (bawaan 5 email per detik) supaya penutupan antrian besar tidak membanjiri SMTP. Polling tiap 5 detik menjadi jaring pengaman bila NOTIFY terlewat (koneksi LISTEN putus disambung ulang).
- **Hak akses:** role `mail_rw` (`db/migrations/04_mail_role.sh`, password `MAIL_DB_PASSWORD`; idempoten): `SELECT` pada outbox, `UPDATE` hanya pada kolom status kirim (`sent_at`, `attempts`, `last_error`, `next_attempt_at`, `failed_at`), dan `SELECT (id, email, username, email_notifications)` pada `users`. Tidak bisa membaca `password_hash`, tiket, atau tabel lain, dan tidak bisa INSERT/DELETE (diuji lewat `SET ROLE`). `notify_ro` tidak diubah. Skrip init hanya jalan pada data directory kosong; untuk DB yang sudah ada jalankan `db/scripts/apply_schema.sql` lalu `04_mail_role.sh` (lihat README).
- **Tautan** di email: `QMS_PUBLIC_ORIGIN` + `/ticket/:queueId` (wajib URL absolut http/https saat worker aktif). Isi berbahasa Indonesia, HTML dan teks, subjek memuat nama antrian dan nomor.

## 6d. Layar publik (fitur 4.5)

Layar tanpa login untuk monitor ruang tunggu: satu kartu per counter, nomor berikutnya, dan dilewati. Kontrak: `api-contract.md` bagian "Layar publik".

- **Kredensial = token di URL.** `queue_displays` (1:1 dengan `queues`, `ON DELETE CASCADE`): `token` unik 24 byte acak (`crypto/rand`, base64url), `upcoming_count` dan `skipped_count` 0 sampai 10 (CHECK). Token disimpan mentah (bukan hash) karena owner perlu melihat URL-nya lagi; karena itu `notify_ro` dan `mail_rw` tidak punya hak apa pun di tabel ini. Token berbentuk salah ditolak tanpa menyentuh database.
- **Data publik minimal.** `DisplaySnapshotResponse` (`dto/display_dto.go`) hanya berisi teks siap tampil. Tes kontrak `TestDisplaySnapshot_PublicResponseKeysAreLocked` mengunci daftar kunci, supaya field baru tidak bocor tanpa sengaja.
- **Room SSE terpisah.** Hub menyiarkan payload event lengkap (id user dan seterusnya) ke semua klien satu room, jadi layar publik tidak boleh ikut room antrian. `sse.DisplayRoom(queueID)` = `-queueID`; `Hub.Notify` mengirim payload lengkap ke room antrian dan **sinyal `display_changed` tanpa data** ke room layar (kecuali `queue_qr_updated`). Tidak ada perubahan pada emitter event yang sudah ada: `counter_updated` (buat, ganti nama, hapus counter), `ticket_*`, `queue_*` semuanya melewati `Hub.Notify`.
- **Cache snapshot** (`service/display_cache.go`): 1 detik per token, satu pemuatan untuk banyak layar yang bersamaan. `Hub.SetBeforeNotify(cache.Invalidate)` membatalkan cache **sebelum** event disebar, jadi klien yang mengambil ulang setelah sinyal tidak mendapat data lama. Pemuatan yang mulai sebelum pembatalan tidak disimpan (penghitung `epoch`). Ganti token atau matikan layar membatalkan cache, jadi token lama langsung 404.
- **Batas.** 20 sambungan SSE layar per antrian (`Hub.RoomSize(DisplayRoom(id))`), selebihnya 429. nginx: `display_zone`, SSE tanpa buffer, dan format log `qms_masked` yang menyamarkan token di path (`map $uri $uri_masked`) untuk `/display/<token>` dan `/api/v1/display/<token>[/event]`; halaman layar memakai `Referrer-Policy: no-referrer` dan `Cache-Control: no-store`.
- **Frontend.** Rute publik `app/display/[token].tsx` (tanpa `AuthGate`), hook `use-display-screen` (snapshot + SSE tanpa login, debounce 150 ms untuk sinyal beruntun, resinkron 45 detik bila stream berdetak dan 3 detik bila tidak, Wake Lock), `DisplayCounterCard` (gaya kartu "Nomor Anda" customer; counter kosong redup), tata letak dinamis 1 sampai 10 kartu (`utils/display.ts`). Status "Menyambung ulang..." muncul bila stream tidak berdetak 30 detik DAN snapshot terakhir lebih dari 10 detik; angka terakhir tetap tampil. Sisi owner: `DisplaySettingsCard` di detail antrian.
- **Belum dibangun:** QR join di layar (kolom `show_qr` sengaja belum ada; tambah lewat `apply_schema.sql`).

## 6c. Foto profil

`PUT/GET/DELETE /users/me/avatar` (api-contract.md). Foto **privat**: hanya `GET /users/me/avatar` yang ada, selalu untuk pemanggil, jadi tidak ada URL publik dan owner antrian tidak melihat foto customer.

- **Pengolahan** (`backend/internal/imageproc`): jenis ditentukan dari isi berkas (`http.DetectContentType`), hanya JPEG, PNG, WebP; dimensi dibaca dari header dulu dan dibatasi 24 megapiksel sebelum decode (anti decompression bomb); crop tengah persegi, perkecil ke 512x512 (CatmullRom, foto kecil tidak diperbesar), encode ulang JPEG dengan kualitas turun bertahap sampai muat 150 KB. Encode ulang membuang EXIF dan GPS; PNG transparan dialasi putih.
- **Penyimpanan**: tabel `user_avatars` (`user_id` PK dan FK `ON DELETE CASCADE`, `data BYTEA` dengan CHECK 150 KB, `updated_at`). Dipilih di Postgres karena otomatis ikut `scripts/backup-prod.sh` (hanya mencadangkan database), tanpa volume atau layanan baru, dan tulis atomik. `GET /users/me` hanya membaca `updated_at` (`avatar_updated_at`), tidak pernah kolom `data`. `notify_ro` tidak punya hak apa pun di tabel ini.
- **Antarmuka `service.AvatarStore`** (`Put/Get/Delete/UpdatedAt`) adalah jalur pindah ke object storage. Ukuran: sekitar 50 KB per foto, jadi 100 ribu foto sekitar 5 GB, 1 juta foto (semua user) sekitar 50 GB. **Ambang pindah: sekitar 100 ribu foto (5 GB di database), atau saat backend dijalankan lebih dari satu instance yang butuh berbagi berkas.** Object storage berarti bucket privat (tanpa versioning), kredensial di env, kebijakan backup sendiri, dan penghapusan objek eksplisit saat akun dihapus.
- **nginx**: lokasi `= /api/v1/users/me/avatar` menaikkan `client_max_body_size` ke 6m hanya di sini (global tetap 1m) dan memakai zona `avatar_zone` 5 r/s per IP. Penolakan 413 dari nginx berbentuk HTML, jadi klien memetakan galat lewat status.
- **Frontend**: `expo-image-picker`. Karena endpoint butuh token, web mengambil foto dengan fetch lalu object URL (`hooks/use-avatar-source.ts`); native memakai `expo-image` dengan header `Authorization`. Web memperkecil foto di browser dulu (`utils/avatar-image.ts`, canvas) supaya unggahan kecil dan orientasi EXIF benar; server tetap memvalidasi.
- **Dev**: container frontend menyimpan `node_modules` di volume anonim, jadi setelah menambah paket jalankan `docker compose up -d --build -V frontend`.

## 7. Batasan dan yang belum dibangun

Belum dibangun:
- Login Google: kolom `users.google_id` ada dan tombol di layar masuk hanya tampilan; backend belum punya OAuth.
- Ganti alamat email (`email_change`) belum dibangun; kolom `purpose` sudah menyiapkannya.
- Observability (metrik, log terpusat, dashboard, alert) dan uji beban.
- Outbox untuk Web Push, migration runner, backup otomatis, HSTS dan Cloudflare Access.
- QR join di layar publik (kolom `show_qr` belum ada); plan langganan selain perpanjangan 1 bulan.
- Dihapus dari produk (31 Okt 2026): `registration_limit`, batas harian QR, batas riwayat, dan penghapusan permanen (riwayat selalu ada).

Sudah dibangun sejak versi sebelumnya dokumen ini: pembatalan oleh customer, hapus nomor oleh owner, counter kustom, sisip posisi, antrian ulang (fallback Skip), catatan, riwayat dan ekspor, layar publik, foto profil, lupa password, email notifikasi, dan pembayaran langganan.

Batasan teknis:
- Hub SSE di memori (lihat 2.7): backend hanya satu instance.
- Deploy demo: `docker-compose.prod.yml` di belakang Cloudflare Tunnel; backend `APP_ENV=production` memvalidasi rahasia dan origin https.
- Push tidak punya outbox; email punya (at-least-once, bisa terkirim dua kali bila proses mati tepat setelah SMTP menerima).
- `ALREADY_JOINED` tidak lagi dikirim. Kode dan penanganan frontend dibiarkan agar tidak merusak klien lama.
