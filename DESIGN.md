# Design — Queue Management System

Dokumen ini menjelaskan arsitektur, keputusan desain, dan batasan sistem. Untuk bentuk request dan response
lihat [api-contract.md](api-contract.md). Untuk alur pengguna lihat [FLOW.md](FLOW.md).

## 1. Gambaran sistem

Sistem antrian untuk layanan tatap muka. Ada dua peran:

- **Owner**: membuat antrian, memanggil nomor berikutnya (Next), dan menutup antrian. Owner harus punya langganan aktif.
- **Customer**: login, bergabung ke antrian lewat tautan, lalu melihat nomor yang sedang dilayani dan posisinya secara real-time.

```
   Browser (Expo web / native)
        │  HTTP (REST, JSON)  +  SSE (fetch stream)
        ▼
   nginx  ── frontend statis (folder dist)
        │  /api/  ──────────►  Backend Go (Gin) :8080
        │                            │
        │                            ▼
        │                      PostgreSQL (qms_db)
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
- Token JWT HS256 berlaku 24 jam. Klaim `sub` berisi user id. Middleware `RequireAuth` memeriksa token di semua
  rute `/api/v1/queues` dan `/api/v1/users/me`.
- Login menerima email atau username. Pesan gagal login tidak membedakan mana yang salah.

Aturan akses:

| Operasi | Siapa boleh |
|---|---|
| Buat, ubah, atur izin, daftar antrian | Owner dengan langganan aktif (daftar tetap ada tanpa langganan, tetapi hanya antrian aktif) |
| Tutup, detail, daftar tiket, Next | Owner antrian |
| Info antrian (`/info`) | Semua user yang login |
| Status antrian dan SSE | Owner atau pemegang tiket di antrian itu |
| Join | Semua user yang login, selama antrian aktif |

Permintaan yang tidak diizinkan dibalas **404**, bukan 403. Dengan begitu keberadaan antrian milik orang lain
tidak bisa ditebak.

### 2.3 Model data

Tabel yang sudah dipakai backend:

| Tabel | Fungsi |
|---|---|
| `users` | Akun (email, username, password hash) |
| `owners` | Penanda owner dan masa berlaku langganan (`subscription_expires_at`) |
| `queues` | Antrian (nama, deskripsi, status, pemilik) |
| `queue_authz` | Izin per antrian (flag `can_*`) |
| `tickets` | Tiket: nomor, `sort_key`, status, waktu selesai |

Tabel yang sudah ada di skema tetapi belum dipakai backend: `payments`, `queue_settings`, `queue_admins`,
`owner_saved_admins`, `counters`, `ticket_notes`, `queue_history`. Tabel-tabel ini disiapkan untuk fitur
yang belum dibangun (lihat bagian 7).

Batasan penting di `tickets`:
- `UNIQUE (queue_id, user_id)`: satu user hanya punya satu tiket per antrian, termasuk setelah tiketnya selesai.
- `UNIQUE (queue_id, sort_key)`: dua tiket tidak boleh punya urutan yang sama, termasuk yang sudah selesai.

### 2.4 Urutan antrian (sort_key)

- Setiap tiket baru mendapat `sort_key` = nilai maksimum `sort_key` di antrian itu + **65536** (gap).
- Maksimum dihitung dari **semua** tiket di antrian, bukan hanya yang `waiting`. Alasannya: `UNIQUE (queue_id, sort_key)`
  berlaku untuk tiket yang sudah selesai juga. Kalau hanya tiket `waiting` yang dihitung, antrian yang sudah habis akan
  menghasilkan kunci yang bentrok dan join gagal terus (ini bug yang pernah terjadi).
- Gap besar memungkinkan penyisipan di tengah di masa depan tanpa menulis ulang banyak baris.
- Next memilih tiket `waiting` dengan `sort_key` terkecil.

### 2.5 Siklus tiket

```
            join                 next
  (baru) ───────────► waiting ───────────► completed
                         │
                         └── (skipped, cancelled: tersedia di skema, belum ada endpoint)
```

- **Join ulang** (user yang sudah punya tiket di antrian itu): backend mengembalikan **tiket yang sudah ada** dengan 200.
  Tidak ada nomor kedua dan tidak ada event baru. Ini menjawab kasus scan QR dua kali atau double-tap.
- **Next** pada antrian tanpa tiket `waiting`: 409 `QUEUE_EMPTY`, tanpa perubahan data.

### 2.6 Nomor yang ditampilkan

- `current_number` = `ticket_number` dari tiket `completed` terbaru.
- `previous_number` = tiket `completed` sebelumnya.
- `next_number` = tiket `waiting` pertama.
- `waiting_ahead` = jumlah tiket `waiting` dengan `sort_key` lebih kecil dari milik requester.

Tampilan customer memakai angka ini langsung dari `GET /status`. Papan owner berbeda: ia memuat daftar tiket
(`GET /tickets`) lalu menurunkan posisi yang sama di frontend (`utils/queue-view.ts`), dengan aturan yang sama dengan di atas.

### 2.7 Real-time (SSE)

- Setiap antrian punya satu *hub* di memori. Klien yang tersambung ke `/queues/:id/event` didaftarkan ke hub itu.
- Service memanggil `notifier` setelah transaksi selesai (join, next, update, close). Hub mengirim event ke semua klien di antrian itu.
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
- `join/[queueId]` juga memakai `required`: user harus login sebelum bergabung.
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

- Backend dijalankan sebagai binary (atau `go run`) di `127.0.0.1:8080`, dengan `.env` yang berisi `DATABASE_URL`,
  `JWT_SECRET`, `PORT`, dan `ALLOWED_ORIGINS`.
- Frontend di-export dengan `npx expo export --platform web` lalu disalin ke folder yang dibaca nginx.
- nginx melayani frontend statis, meneruskan `/api/` ke backend, memakai rute SSE tanpa buffer, dan mengurus TLS.
  Konfigurasinya ada di `deploy/nginx/`.
- Docker disiapkan sebagai jalur kedua. Setiap layanan punya Dockerfile dan `Dockerfile.dockerignore`:
  `db/`, `backend/`, `frontend/`, dan `deploy/nginx/`. Berkas ini belum pernah di-build dan belum ada `docker-compose.yml`.
- Pada container, frontend tidak berjalan sebagai server. Image frontend hanya berisi hasil build (`/dist`) yang diisi ke volume.
  Container nginx membaca volume itu dan meneruskan `/api/` ke container backend (nama service `backend`, port 8080).
- Nginx di container hanya HTTP. TLS diurus di luar container atau ditambahkan kemudian.
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
| End-to-end | Playwright (tidak dijalankan secara default) | `frontend/playwright` |

Ekspektasi tes diturunkan dari product overview (`documentation/docs/product-overview.md`), bukan dari kode.

## 7. Batasan dan yang belum dibangun

Yang belum ada di backend (lihat product overview):
- Skip tiket, notifikasi ke customer berikutnya, pembatalan oleh customer, hapus customer oleh owner.
- Counter dan admin per counter, penyisipan di posisi tertentu, antrian ulang (requeue).
- QR dinamis dan batas harian, riwayat dan ekspor, catatan antrian, Google login.
- Pengelolaan admin antrian dan pembayaran langganan.

Di frontend, tombol dan layar untuk fitur di atas sudah ada tetapi belum tersambung (TODO di kode).

Batasan teknis:
- Hub SSE di memori (lihat 2.7).
- `ALREADY_JOINED` tidak lagi dikirim. Kode dan penanganan frontend dibiarkan agar tidak merusak klien lama.
