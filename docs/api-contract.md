# API Contract — Queue Management System

Kontrak HTTP dan SSE antara frontend dan backend. Sumber kebenaran ada di kode backend
(`backend/internal/router`, `backend/internal/handler`, `backend/internal/dto`); dokumen ini
merangkum bentuk request dan response yang dipakai frontend.

## Ringkasan

| Hal | Nilai |
|---|---|
| Base path | `/api/v1` |
| Format | JSON, `Content-Type: application/json` |
| Nama field | `snake_case` |
| Field kosong | `null` (bukan string kosong atau `0`) |
| Autentikasi | `Authorization: Bearer <token>` (JWT RS256 diterbitkan `auth-service`, berlaku 24 jam, klaim `sub` = user id, `iat`). Diverifikasi secara lokal dengan kunci publik |
| Waktu | RFC 3339 (`2026-10-17T16:29:57.635239+07:00`) |

## Amplop respons

Sukses: objek dengan kunci `data`.

```json
{ "data": { "action": "list_queues", "queues": [ ... ] } }
```

Gagal: objek dengan kunci `error` yang berisi `code` (string tetap) dan `message` (teks untuk manusia).

```json
{ "error": { "code": "NOT_FOUND", "message": "queue not found" } }
```

Beberapa endpoint (join, next) membalas objek tiket langsung di dalam `data`, tanpa `action`.

## Kode error

| HTTP | `code` | Arti |
|---|---|---|
| 400 | `BAD_REQUEST` | Body JSON tidak valid, field wajib kosong, atau `queue_id` bukan angka |
| 401 | `UNAUTHORIZED` | Token tidak ada, tidak valid, kedaluwarsa, atau user tidak ditemukan |
| 402 | `SUBSCRIPTION_INACTIVE` | Langganan owner tidak aktif (membuat, mengubah, atau mengatur izin antrian) |
| 403 | `EMAIL_NOT_VERIFIED` | Email akun belum diverifikasi (membuat antrian, atau join dengan nomor baru). Hanya ada bila verifikasi email menyala |
| 404 | `NOT_FOUND` | Antrian tidak ada, atau requester tidak punya hak melihatnya |
| 409 | `CONFLICT` | Email atau username sudah terdaftar (register) |
| 409 | `QUEUE_CLOSED` | Antrian sudah ditutup (join) |
| 409 | `QUEUE_EMPTY` | Tidak ada tiket yang menunggu (next) |
| 409 | `ALREADY_JOINED` | Join ulang (tiket aktif) mengembalikan tiket yang sudah ada dengan 200, jadi `POST /join` tidak pernah menjawab kode ini walau handler memetakannya; kode ini hanya muncul di `insert` (akun sudah punya tiket aktif) |
| 409 | `AMBIGUOUS_ACCOUNT` | Username pada `insert` dipakai lebih dari satu akun |
| 409 | `NOTHING_TO_SKIP` / `NOTHING_TO_RECALL` | Counter tidak sedang melayani nomor |
| 409 | `RECALL_TOO_SOON` / `ALREADY_RECALLED` | Recall belum boleh (kurang dari 3 detik) atau sudah dipakai |
| 409 | `COUNTER_STATE_CHANGED` | Next/Skip/Recall: nomor counter berbeda dari `expected_ticket_id` (counter baru saja dimajukan dari perangkat lain); tidak ada data yang berubah |
| 409 | `DUPLICATE_COUNTER_NAME` | Nama counter sudah dipakai di antrian itu |
| 409 | `COUNTER_LIMIT` | Antrian sudah punya 10 counter |
| 409 | `DEFAULT_COUNTER` | Counter default tidak bisa dihapus |
| 400 | `WEAK_PASSWORD` | Password di luar 8 karakter sampai 72 byte |
| 400 | `INVALID_TOKEN` | Token reset password atau verifikasi email tidak ada, kedaluwarsa, atau sudah dipakai |
| 429 | `TOO_MANY_REQUESTS` | Kirim ulang email verifikasi melebihi 3 per jam per akun |
| 429 | `TOO_MANY_DISPLAYS` | Terlalu banyak sambungan layar publik untuk satu antrian |
| 503 | `BUSY` | Terlalu banyak pengiriman email lupa password berjalan |
| 413 | `PAYLOAD_TOO_LARGE` | Foto profil lebih dari 5 MB |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | Foto profil bukan JPEG, PNG, atau WebP |
| 503 | `SERVICE_UNAVAILABLE` | Sambungan SSE (antrian atau layar publik) ditolak karena hub berhenti atau penuh |
| 409 | `QR_REPLACE_NOT_ALLOWED` | Penggantian kode QR hanya untuk mode statis |
| 400 | `QR_TYPE_INVALID` | `type` QR bukan `static`/`dynamic` |
| 404 | `QR_INVALID` | Kode join tidak valid, sudah diganti, atau antrian tidak ada |
| 410 | `QR_EXPIRED` | Kode join dinamis sudah kedaluwarsa |
| 500 | `INTERNAL_ERROR` | Kesalahan server |
| 503 | `RETRY_EXHAUSTED` | Join gagal setelah beberapa percobaan karena banyak permintaan bersamaan |

Catatan: 404 dipakai juga untuk "bukan milik kamu". Ini disengaja agar keberadaan antrian orang lain tidak bocor.

## Autentikasi

### POST /api/v1/users/register
Tanpa token.

Request:
```json
{ "email": "owner@contoh.com", "username": "owner1", "password": "rahasia123" }
```
Semua field wajib. `email` dan `username` tidak boleh mengandung tag HTML. `password` 8 karakter sampai 72 byte (batas bcrypt).

200:
```json
{ "data": { "token": "eyJhbGciOi..." } }
```
Error: 400 `BAD_REQUEST`, 400 `WEAK_PASSWORD` (di luar 8 karakter sampai 72 byte), 409 `CONFLICT`, 500 `INTERNAL_ERROR`.

Bila verifikasi email menyala, akun baru langsung dikirimi email tautan verifikasi di latar belakang (kegagalan kirim hanya masuk log; registrasi tetap 200). Lihat `POST /users/verify-email`.

Token memuat klaim `sub`, `iat`, dan `exp` (24 jam). Token yang `iat`-nya lebih lama dari `password_changed_at` user ditolak 401 (lihat reset password di bawah). Token tanpa `iat` (terbitan sebelum fitur ini) tetap sah sampai `exp`.

### POST /api/v1/users/login
Tanpa token. Menerima email atau username lewat `identifier`. Field `email` tetap diterima sebagai alias.

Request:
```json
{ "identifier": "owner@contoh.com", "password": "rahasia123" }
```

200: `{ "data": { "token": "..." } }`
Error: 400 `BAD_REQUEST` (identifier dan email kosong), 401 `UNAUTHORIZED` (kredensial salah; pesan tidak membedakan mana yang salah).

### POST /api/v1/users/google
Tanpa token. Menukar **ID token Google** dengan token login QMS. Dilayani `auth-service` dan hanya ada bila `GOOGLE_CLIENT_IDS` terisi (selain itu 404). Klien mendapat ID token dari Google (web: `expo-auth-session`; native: dev build).

Request: `{ "id_token": "eyJhbGciOi..." }` (wajib, maks 8192 karakter).

Server memeriksa ID token sendiri: tanda tangan RS256 dengan kunci publik Google, `iss` Google, `aud` salah satu `GOOGLE_CLIENT_IDS`, `exp`/`iat` (toleransi 60 detik), dan `email_verified`. Email dan `sub` dari klien tidak pernah dipercaya. Akun dicocokkan lewat `google_id` (klaim `sub`), bukan email.

| Kondisi | Hasil |
|---|---|
| Akun sudah tertaut ke `google_id` ini | **200** `{ "data": { "token": "..." } }` |
| Email belum terdaftar | Akun baru dibuat tanpa password, `email_verified_at` terisi (Google sudah menjaminnya), username dari nama Google (maks 50 karakter, tanpa `<` `>`). **200** dengan token |
| Email sudah terdaftar **dengan password** (huruf besar/kecil diabaikan) | **409** `GOOGLE_LINK_REQUIRED`. Akun **tidak** ditautkan otomatis (mencegah pengambilalihan akun lewat email). Memuat `error.link_token` |
| Email terdaftar tetapi akun itu sudah tertaut ke akun Google lain, atau tidak punya password | 409 `CONFLICT` |

409 `GOOGLE_LINK_REQUIRED`:
```json
{ "error": { "code": "GOOGLE_LINK_REQUIRED", "message": "...", "link_token": "eyJhbGciOi..." } }
```

Error lain: 400 `BAD_REQUEST` (body rusak), 401 `UNAUTHORIZED` (ID token tidak sah), 403 `GOOGLE_EMAIL_UNVERIFIED` (Google tidak menjamin emailnya), 503 `GOOGLE_UNAVAILABLE` (kunci publik Google tidak bisa diambil), 503 `AUTH_UNAVAILABLE` (dari nginx bila auth-service mati). Respons memuat `Cache-Control: no-store`. Dibatasi laju per IP di nginx (`auth_zone`).

### POST /api/v1/users/google/link
Tanpa token. Menautkan akun Google ke akun terdaftar setelah pemiliknya membuktikan password.

Request: `{ "link_token": "...", "password": "rahasia123" }` (`link_token` dari 409 di atas).

200: `{ "data": { "token": "..." } }`. Akun tertaut ke `google_id` itu dan emailnya ditandai terverifikasi. Password lama tetap berlaku, jadi akun bisa login dengan keduanya.
Error: 400 `INVALID_TOKEN` (link token tidak sah, kedaluwarsa, sudah dipakai, atau sudah 5 kali salah password), 401 `UNAUTHORIZED` (password salah; sama dengan login gagal), 409 `CONFLICT` (akun sudah tertaut ke akun Google lain), 503 `AUTH_UNAVAILABLE`.

**Link token:** JWT RS256 berumur **5 menit** dengan klaim `purpose: "google_link"`, `gid` (id Google), dan `jti` acak. Sekali pakai; setelah 5 kali salah password token dibakar. Ia ditandatangani kunci yang sama dengan token login dan memuat `sub`, tetapi **bukan token login**: `auth-service` dan backend menolak token ber-`purpose` (401). Jangan disimpan di storage dan jangan dicatat di log.

### POST /api/v1/users/forgot-password
Tanpa token. Meminta tautan untuk membuat password baru; tautan dikirim ke email akun lewat `mail-service` (templat `password-reset`).

Request: `{ "email": "budi@contoh.com" }`

**202** dengan pesan yang selalu sama untuk email berformat sah, terdaftar atau tidak, dan pengiriman berjalan asinkron, jadi respons tidak membocorkan ada-tidaknya akun:
```json
{ "data": { "message": "Jika email terdaftar, tautan untuk membuat password baru sudah dikirim." } }
```
Error: 400 `BAD_REQUEST` (badan rusak, email kosong atau tidak sah, mengandung tag HTML), 503 `BUSY` (terlalu banyak pengiriman berjalan), 429 dari nginx (1 permintaan/detik per IP, burst 5; tanpa badan JSON).

Aturan: token acak 32 byte (`crypto/rand`, base64url) yang hanya disimpan sebagai hash SHA-256 di `user_action_tokens`; berlaku 30 menit, sekali pakai; permintaan baru membatalkan token lama yang belum terpakai; paling banyak 3 token per akun per jam (kelebihannya diam-diam tidak dikirim, tetap 202). Tautan: `${PUBLIC_APP_URL}/reset-password?token=...`. Galat pengiriman email hanya masuk log server.

### POST /api/v1/users/reset-password
Tanpa token. Memasang password baru dari tautan email.

Request: `{ "token": "<dari tautan>", "password": "passwordbaru1" }`

200: `{ "data": { "status": "ok" } }`. Password di-bcrypt, `users.password_changed_at` diisi (semua sesi lama dicabut), token ditandai terpakai bersama token reset lain milik user itu. Semuanya satu transaksi.

Error: 400 `WEAK_PASSWORD` (password diperiksa **sebelum** token disentuh, jadi token tidak habis), 400 `INVALID_TOKEN` (tidak ada, kedaluwarsa, atau sudah dipakai; satu pesan untuk ketiganya), 400 `BAD_REQUEST`, 429 dari nginx.

### POST /api/v1/users/verify-email
Tanpa token (kredensialnya token di tautan email). Dilayani `auth-service`; rute ini (bersama `forgot-password`, `reset-password`, dan `me/verify-email/resend`) hanya dipasang bila `MAIL_SERVICE_URL` terisi. Di auth-service `EMAIL_VERIFICATION_REQUIRED` kosong = ikut ada-tidaknya `MAIL_SERVICE_URL`, dan `true` tanpa mail-service membuat auth-service gagal start. Di backend, nilai kosong berarti `true` (guard menyala); nilainya harus sama di kedua service.

Request: `{ "token": "<dari tautan>" }`

200: `{ "data": { "status": "ok" } }` (header `Cache-Control: no-store`). Mengisi `users.email_verified_at` dan menandai semua token verifikasi milik user itu terpakai, satu transaksi.

Error: 400 `INVALID_TOKEN` (tidak ada, kedaluwarsa, atau sudah dipakai; satu pesan untuk ketiganya), 400 `BAD_REQUEST`.

Email konfirmasi: bila token ini membuat akun yang **belum** terverifikasi menjadi terverifikasi, `auth-service` mengirim email templat `email-verified` lewat `mail-service`. Pengiriman best effort dan asinkron: kegagalan hanya masuk log dan tidak pernah mengubah respons 200 di atas. Tidak dikirim bila akun sudah terverifikasi sebelumnya (mis. dari Google) atau bila `MAIL_SERVICE_URL` kosong.

Aturan: token sekali pakai berlaku 24 jam, hanya hash SHA-256 yang disimpan di `user_action_tokens` (`purpose = 'email_verify'`). Tautan: `${PUBLIC_APP_URL}/verify-email?token=...`, email lewat `mail-service` (templat `email-verify`).

### POST /api/v1/users/me/verify-email/resend
Butuh token. Kirim ulang tautan verifikasi. 202 `{ "data": { "message": "Jika email belum terverifikasi, tautan verifikasi sudah dikirim." } }`, juga bila akun sudah terverifikasi (tidak ada yang dikirim). Token lama yang belum terpakai dibatalkan. Error: 429 `TOO_MANY_REQUESTS` (maksimal 3 per jam per akun), 401 `UNAUTHORIZED`.

Selama belum terverifikasi: `POST /queues` dan `POST /join` (hanya untuk nomor baru; tiket yang sudah ada tetap dikembalikan) dijawab 403 `EMAIL_NOT_VERIFIED`. Status dibaca dari database tiap permintaan, jadi berlaku tanpa login ulang.

### GET /api/v1/users/me
Butuh token. Token yang diterbitkan sebelum password terakhir diganti dijawab 401 `UNAUTHORIZED`.

200:
```json
{ "data": { "id": 1, "email": "owner@contoh.com", "username": "owner1", "is_owner": true, "subscription_expires_at": "2026-12-04T02:01:06Z", "avatar_updated_at": null, "email_notifications": true, "email_verified": true } }
```
`email_verified` = email sudah diverifikasi; klien menampilkan banner verifikasi di Profil bila `false`. Selalu `true` bila verifikasi email dimatikan. `email_notifications` = email notifikasi tiket menyala (default `true`). `is_owner` = true bila user punya baris di tabel `owners`. `subscription_expires_at` hanya terisi untuk owner (`null` untuk customer). `avatar_updated_at` = waktu foto profil terakhir diganti, `null` bila belum punya foto (klien memakainya sebagai penanda ada foto dan pemecah cache).

### PUT /api/v1/users/me
Butuh token. Mengganti username user yang login. Email tidak bisa diubah di sini (field lain diabaikan).

Body:
```json
{ "username": "Nama Baru" }
```
Spasi di tepi dibuang. Username tidak harus unik.

200: profil terbaru, bentuknya sama dengan `GET /users/me`.

Error: 400 `BAD_REQUEST` (kosong, lebih dari 50 karakter, atau mengandung tag HTML seperti `<` dan `>`), 401 `UNAUTHORIZED` (tanpa token atau token tidak valid).

### PUT /api/v1/users/me/preferences
Butuh token. Menyalakan atau mematikan email notifikasi (tiket: join, batal, dihapus owner, selesai dilayani; owner: antrian miliknya ditutup). Berlaku untuk kejadian berikutnya: email yang sudah masuk antrian kirim tetap terkirim, dan email yang terlewat saat sakelar mati tidak dikirim belakangan.

Body:
```json
{ "email_notifications": false }
```
200: profil terbaru, bentuknya sama dengan `GET /users/me`. Error: 400 `BAD_REQUEST` (field hilang, `null`, atau bukan boolean), 401 `UNAUTHORIZED`.

### PUT /api/v1/users/me/avatar
Butuh token. Mengunggah atau mengganti foto profil. `multipart/form-data` dengan field `file`.

Server menentukan jenis dari **isi berkas** (bukan nama atau header klien). Diterima: JPEG, PNG, WebP, maksimal 5 MB dan 24 megapiksel. Hasilnya dipotong persegi di tengah, diperkecil ke 512x512 (foto lebih kecil tidak diperbesar), dan disimpan sebagai JPEG (maksimal 150 KB). Pengodean ulang membuang EXIF dan metadata lain, termasuk lokasi GPS. Bagian transparan jadi putih. Satu foto per user; unggahan baru menggantikan yang lama.

200: profil terbaru, bentuknya sama dengan `GET /users/me` (`avatar_updated_at` terisi).

Error: 400 `BAD_REQUEST` (field `file` tidak ada, atau berkas rusak atau dimensinya terlalu besar), 413 `PAYLOAD_TOO_LARGE` (lebih dari 5 MB; bila ditolak nginx, bodynya HTML dan bukan JSON), 415 `UNSUPPORTED_MEDIA_TYPE` (bukan JPEG, PNG, atau WebP), 401 `UNAUTHORIZED`. Dibatasi laju 5 permintaan per detik per IP (nginx).

### GET /api/v1/users/me/avatar
Butuh token. Foto profil **milik sendiri**; tidak ada URL publik dan tidak ada rute untuk melihat foto user lain. 200 `image/jpeg` dengan `ETag`, `Cache-Control: private, no-cache`, dan `X-Content-Type-Options: nosniff`; kirim `If-None-Match` untuk 304. 404 `NOT_FOUND` bila belum ada foto. Karena butuh header `Authorization`, di web foto diambil dengan fetch lalu dijadikan object URL (bukan `<img src>` langsung).

### DELETE /api/v1/users/me/avatar
Butuh token. Menghapus foto profil. Idempoten: 204 tanpa body, juga bila belum ada foto.

### GET /api/v1/users/me/tickets
Butuh token. Tiket milik user di semua antrian.

200:
```json
{
  "data": {
    "action": "list_my_tickets",
    "tickets": [
      {
        "id": 121, "queue_id": 122, "ticket_number": 2, "ticket_prefix": "A", "sort_key": 131072, "user_id": 3,
        "status": "waiting", "skip_count": 0, "last_notified_at": null,
        "created_at": "2026-10-17T16:29:57+07:00", "completed_at": null, "cancelled_at": null,
        "queue_name": "Loket A", "queue_status": "active", "current_number": 1
      }
    ]
  }
}
```
`current_number` = `ticket_number` yang sedang dilayani di antrian itu (tiket `currently_served`), atau null.
Satu baris per antrian: bila user punya tiket aktif (`currently_served` lalu `waiting`) di antrian itu, tiket aktif yang tampil; bila tidak, tiket terakhirnya.
`ticket_prefix` = awalan nomor milik antrian (bukan kolom `tickets`). Label tampilan = `ticket_prefix` + `ticket_number`, mis. `A2`.

### GET /api/v1/users/me/history?limit=&cursor=
Butuh token. Riwayat tiket milik user yang login (customer), satu baris per tiket, terbaru dulu. Tidak ada parameter user: hanya tiket milik token. Paginasi sama dengan riwayat owner (`cursor` buram, `limit`; lihat bagian Riwayat antrian).

200:
```json
{ "data": { "action": "list_my_history",
  "tickets": [ { "ticket_id": 56, "queue_id": 54, "queue_name": "Loket", "ticket_prefix": "A", "ticket_number": 3,
    "outcome": "completed", "queue_closed": true, "skip_count": 0,
    "joined_at": "...", "called_at": "...", "completed_at": "...", "cancelled_at": null, "counter_name": null } ],
  "next_cursor": null } }
```
Sengaja tanpa catatan owner dan tanpa data user lain.

## Antrian (owner)

Semua endpoint di bagian ini butuh token. Kecuali `GET /queues/:id` (juga untuk pemegang tiket) dan `GET /queues/:id/status`,
hanya owner antrian yang boleh memakainya.

### POST /api/v1/queues
Membuat antrian. Butuh langganan owner aktif.

Request:
```json
{ "queue_name": "Loket A", "queue_description": "Pelayanan umum", "ticket_prefix": "A" }
```
`queue_name` wajib. `queue_description` boleh kosong. `ticket_prefix` opsional: 1-3 huruf/angka (huruf kecil diubah jadi besar), kosong = `A`. Selain itu 400 `BAD_REQUEST`. Nomor tampil sebagai `<prefix>-<nomor 3 digit>`, mis. `VIP-007`.

201:
```json
{ "data": { "action": "create_queue", "queue": { ...QueueResponse } } }
```
Antrian baru otomatis punya satu counter default dan satu baris `queue_settings`.

Error: 400 `BAD_REQUEST`, 403 `EMAIL_NOT_VERIFIED` (diperiksa lebih dulu daripada langganan), 402 `SUBSCRIPTION_INACTIVE`, 500.

### GET /api/v1/queues
Daftar antrian milik owner. Bila langganan tidak aktif, hanya antrian yang masih aktif yang dikembalikan (tidak ada 402).

200: `{ "data": { "action": "list_queues", "queues": [ ...QueueResponse ] } }`

### GET /api/v1/queues/:queue_id
Detail antrian. Owner mendapat `QueueResponse` lengkap; pemegang tiket di antrian itu mendapat `PublicQueueResponse` (tanpa `owner_id`). Pengguna lain: 404. (Endpoint `GET /queues/:id/info` lama sudah tidak ada; gantinya endpoint ini.)

200: `{ "data": { "action": "get_queue", "queue": {...} } }`
Error: 400, 404 `NOT_FOUND`.

### PUT /api/v1/queues/:queue_id
Ubah nama dan deskripsi. Butuh langganan aktif dan hak owner.

Request: `{ "queue_name": "Loket B", "queue_description": "...", "ticket_prefix": "B" }` (`ticket_prefix` opsional; kosong = tidak diubah; aturan sama dengan buat antrian. Mengubahnya mengubah tampilan semua nomor di antrian itu, termasuk yang sudah terbit.)
200: `{ "data": { "action": "update_queue", ... } }`
Error: 400, 402 `SUBSCRIPTION_INACTIVE`, 404.

### POST /api/v1/queues/:queue_id/close
Menutup antrian. Tidak bisa dibuka lagi. Customer tidak bisa join setelah ditutup. Nomor yang masih `currently_served` ikut menjadi `completed` (`completed_at` terisi) dalam transaksi yang sama, sehingga tidak ada tiket yang menggantung. `closed_at` diisi dengan waktu penutupan; antrian yang ditutup sebelum perbaikan ini diberi perkiraan oleh `db/scripts/apply_schema.sql` (aktivitas tiket terakhir, atau `created_at` bila tanpa tiket).

200: `{ "data": { "action": "close", ... } }`
Error: 400, 404.

Efek samping: bila `users.email_notifications` owner menyala, trigger database menulis satu baris outbox `queue-closed` (paling banyak satu per antrian) dan `mail-service` mengirim email "antrian ditutup" ke owner. Nomor yang sedang dilayani yang ikut menjadi `completed` tetap memicu email "selesai dilayani" bagi pemegangnya, sesuai sakelar masing-masing. Gagal kirim tidak memengaruhi respons.

### POST /api/v1/queues/:queue_id/next
Memanggil tiket `waiting` pertama dan menjadikannya `currently_served` (nomor yang sedang dilayani) di sebuah counter. Nomor yang sedang dilayani counter itu sebelumnya otomatis menjadi `completed` (`completed_at` terisi) dalam transaksi yang sama. Hanya owner.

Body opsional (juga untuk `skip` dan `recall`): `{ "counter_id": 0, "expected_ticket_id": 121 }`. `counter_id` 0 atau kosong = counter default; counter lain harus milik antrian ini. `expected_ticket_id` = id nomor yang dilihat perangkat sebagai nomor counter itu (0 = counter kosong); bila berbeda dari keadaan sebenarnya, 409 `COUNTER_STATE_CHANGED` tanpa perubahan data (mencegah dua perangkat memajukan counter yang sama sekaligus). Tanpa field itu (klien lama) tidak ada pengecekan. Body kosong atau rusak dianggap counter default.

200: objek tiket langsung di dalam `data`, dengan `status` = `"currently_served"`.
```json
{ "data": { "id": 121, "queue_id": 122, "ticket_number": 2, "status": "currently_served",
            "called_counter_id": 11, "called_counter_name": null, "called_at": "...", ... } }
```
Error: 404 `NOT_FOUND` (bukan owner), 409 `QUEUE_EMPTY` (tidak ada yang menunggu), 409 `COUNTER_STATE_CHANGED`.

### GET /api/v1/queues/:queue_id/settings/skip
Pengaturan Skip antrian. Hanya owner. Respons: `{ "data": { "fallback": "remove|back|after_n", "n": null|int, "max_returns": int } }`. Default `remove`, `n` null, `max_returns` 1.

### PUT /api/v1/queues/:queue_id/settings/skip
Body `{ "fallback": "remove|back|after_n", "n": int, "max_returns": int }`. Hanya owner dengan langganan aktif pada antrian yang belum ditutup. `n` wajib ≥ 1 untuk `after_n` dan diabaikan untuk mode lain. `max_returns` ≥ 1: berapa kali satu tiket boleh dikembalikan sebelum menjadi `skipped` permanen.
- `remove`: nomor langsung `skipped`.
- `back`: kembali `waiting` di urutan paling belakang.
- `after_n`: kembali `waiting` setelah N tiket yang sedang menunggu (dihitung setelah tiket berikutnya dipanggil); bila yang menunggu kurang dari N, ke paling belakang. Bila jarak `sort_key` habis, tiket waiting dinomori ulang otomatis.
Error: 400 `BAD_REQUEST` (mode tidak dikenal, N atau `max_returns` < 1), 402 `SUBSCRIPTION_INACTIVE`, 403 (bukan owner), 409 `QUEUE_CLOSED`, 404 `NOT_FOUND`.

### POST /api/v1/queues/:queue_id/recall
Owner memanggil ulang nomor yang sedang dilayani counter (body sama seperti `next`). Hanya push ke pemiliknya (status tidak berubah). Aktif 3 detik setelah nomor dipanggil (Next/Skip) dan hanya sekali per pemanggilan. Error 409: `NOTHING_TO_RECALL`, `RECALL_TOO_SOON`, `ALREADY_RECALLED`, `COUNTER_STATE_CHANGED`. Bukan owner antrian: 404.

### GET /api/v1/queues/:queue_id/tickets/:ticket_id/note
Catatan owner untuk satu nomor tiket. Hanya owner antrian. Dipakai saat popup Append Note dibuka, supaya catatan yang sudah ada terisi lebih dulu.

200: `{ "data": { "ticket_id": 121, "note": "...", "created_at": "...", "updated_at": "..." } }`. Nomor tanpa catatan: `note` = null dan tanggalnya null (bukan 404).
Error: 404 `NOT_FOUND` (bukan owner, atau tiket bukan milik antrian ini).

### PUT /api/v1/queues/:queue_id/tickets/:ticket_id/note
Membuat catatan bila belum ada, atau menimpanya bila sudah ada (maksimal satu catatan per nomor). Body `{ "note": "..." }`. Spasi di awal dan akhir dipangkas.

201: catatan baru dibuat. 200: catatan lama ditimpa. Respons sama seperti GET.
Error: 400 `BAD_REQUEST` (catatan kosong setelah dipangkas, atau lebih dari 1000 karakter; catatan lama tidak berubah), 404 `NOT_FOUND` (bukan owner, atau tiket bukan milik antrian ini).

Catatan tidak pernah ikut dalam respons tiket dan tidak terlihat oleh customer.

### POST /api/v1/queues/:queue_id/insert
Owner menyisipkan akun terdaftar ke posisi tertentu di urutan tunggu (US-10 / 4.12). Hanya owner antrian.

Body: `{ "identifier": "email atau username", "position": 2 }`. `identifier` dipangkas spasi dan wajib diisi. `position` bilangan bulat minimal 1, dihitung dari tiket `waiting` (1 = berikutnya). Posisi melebihi panjang antrian masuk di paling belakang.

201: objek tiket baru (`status` = `"waiting"`), dengan `ticket_number` berikutnya seperti join biasa. Nomor yang sedang dilayani tidak terganggu.

Error:
- 400 `BAD_REQUEST`: `identifier` kosong, `position` kurang dari 1, atau body tidak valid.
- 404 `NOT_FOUND`: bukan owner, antrian tidak ada, atau akun tidak ditemukan.
- 409 `AMBIGUOUS_ACCOUNT`: lebih dari satu akun memakai username itu. Gunakan email.
- 409 `ALREADY_JOINED`: akun sudah punya tiket aktif di antrian ini.
- 409 `QUEUE_CLOSED`.
- 503 `RETRY_EXHAUSTED`: gagal setelah beberapa kali percobaan, biasanya karena antrian sangat padat.

Akun yang tiket lamanya sudah selesai boleh disisipkan lagi, dengan nomor baru.

### POST /api/v1/queues/:queue_id/cancel
Customer membatalkan nomornya sendiri (4.9). Login customer, tanpa body. Tiket aktif (`waiting` atau `currently_served`) milik user dari token dibatalkan; bila nomor itu sedang dilayani, waiting teratas naik ke `currently_served`. Nomor tidak dihapus: statusnya menjadi `cancelled`, dan user boleh bergabung lagi dengan nomor baru.

200: objek tiket setelah dibatalkan (`status` = `"cancelled"`, `cancelled_at` terisi). Tanpa tiket aktif, atau sudah dibatalkan: 200 dengan keadaan tiket terakhir (no-op, klik ganda aman).
Error: 404 `NOT_FOUND` (antrian tidak ada, atau user belum pernah punya tiket di antrian itu), 409 `QUEUE_CLOSED`.

### POST /api/v1/queues/:queue_id/tickets/:ticket_id/remove
Owner menghapus nomor customer dari antrian (4.10). Hanya owner antrian. Perilakunya seperti skip tanpa fallback: nomor keluar dari urutan tunggu dan tidak dikembalikan ke `waiting`; `skip_count` tidak naik. Nomor yang sedang dilayani membuat waiting teratas naik ke `currently_served`. Status akhirnya `deleted`, dan nomor itu tidak tampil di daftar `skipped`.

200: objek tiket setelah dihapus (`status` = `"deleted"`). Tiket yang sudah tidak aktif (`skipped`, `cancelled`, `deleted`, `completed`): 200 dengan keadaan tiket sekarang (no-op).
Error: 404 `NOT_FOUND` (bukan owner, atau tiket milik antrian lain), 409 `QUEUE_CLOSED`, 400 `BAD_REQUEST` (id tidak valid).

Push ke customer untuk penghapusan belum ada; akan ditambahkan di pekerjaan berikutnya.

### Web Push (perlu login)
- `GET /api/v1/push/public-key` → `{ public_key }` (kunci VAPID publik).
- `POST /api/v1/push/subscribe` body `{ endpoint, keys: { p256dh, auth } }`; endpoint wajib `https`, 400 bila tidak. Endpoint yang sama dipindah ke user yang sedang login.
- `POST /api/v1/push/unsubscribe` body `{ endpoint }`.
Rute `/push` hanya ada bila `VAPID_PUBLIC_KEY` diset. Push dikirim `notify-service` (LISTEN `queue_events`): nomor dipanggil → pemilik "Giliran Anda" dan tiket waiting terdepan "Tinggal 1 nomor lagi"; recall → pemilik saja; skip → pemilik nomor yang dilewati ("dilewati" atau "dilewati dan dikembalikan ke antrian").

### Push aplikasi Android / FCM (perlu login)
- `POST /api/v1/push/fcm/register` body `{ token: string, platform: "android" }` → `204` tanpa body. `token` adalah token FCM perangkat (`getDevicePushTokenAsync` di expo-notifications). Token yang sama dipindah ke user yang sedang login (sama seperti endpoint Web Push). `400 BAD_REQUEST` bila token kosong atau `platform` bukan `android`.
- `POST /api/v1/push/fcm/unregister` body `{ token: string }` → `204` tanpa body. Hanya menghapus token milik user yang login; token milik akun lain tidak tersentuh. `400 BAD_REQUEST` bila token kosong.
Rute `/push/fcm` hanya ada bila `FCM_PROJECT_ID` diset di backend. Isi notifikasi dan pemicunya sama dengan Web Push; `notify-service` mengirim lewat FCM HTTP v1 bila `FCM_CREDENTIALS_FILE` terpasang, dan menghapus token yang dijawab `UNREGISTERED` oleh FCM. Ketuk notifikasi membuka `data.url` (mis. `/ticket/12`).

### POST /api/v1/queues/:queue_id/skip
Melewati nomor yang sedang dilayani counter (tiket `currently_served`; body sama seperti `next`) lalu langsung memanggil tiket `waiting` pertama ke counter yang sama, dalam satu transaksi. Hanya owner. `skip_count` +1 dan `completed_at` dikosongkan. Nasib tiket yang dilewati mengikuti pengaturan Skip antrian (lihat `settings/skip`): `remove` atau batas `max_returns` tercapai menjadi `skipped` permanen (`called_at` tetap); `back`/`after_n` mengembalikannya ke `waiting` di posisi baru (`called_at` dan `called_counter_id` dikosongkan).

200: objek tiket yang baru dipanggil (sama seperti Next), `status` = `"currently_served"`.
Error: 404 `NOT_FOUND` (bukan owner), 409 `NOTHING_TO_SKIP` (belum ada nomor yang dilayani), 409 `QUEUE_EMPTY` (tidak ada yang menunggu; tidak ada perubahan, nomor tetap dilayani), 409 `COUNTER_STATE_CHANGED`.

### GET /api/v1/queues/:queue_id/tickets
Semua tiket antrian, urut `sort_key`. Hanya owner. Maksimal 500 baris.

Query opsional: `status` (`waiting`, `currently_served`, `completed`, `cancelled`, `skipped`, `deleted`), boleh diulang atau dipisah koma. `deleted` = nomor yang dihapus owner (4.10).

200:
```json
{ "data": { "action": "list_tickets", "queue_id": 122, "tickets": [ ...OwnerTicketResponse ] } }
```
`OwnerTicketResponse` = `TicketResponse` ditambah `note` dan `note_updated_at` (catatan owner; null bila tidak ada). Hanya endpoint ini yang memuat catatan.
Error: 400, 404.

### GET /api/v1/queues/:queue_id/qr
Status QR antrian untuk owner. Token statis dibuat lazy saat pertama kali diminta.

200:
```json
{
  "data": {
    "action": "queue_qr",
    "queue_id": 122,
    "type": "static",
    "join_url": "https://qms.example.com/join?t=K7M2QX",
    "expires_at": null
  }
}
```
`type` = `static` | `dynamic`. `expires_at` hanya untuk dinamis (detik Unix).

Error: 404 `NOT_FOUND` (bukan owner).

### PUT /api/v1/queues/:queue_id/qr/type
Mengubah mode QR. Body `{ "type": "static" | "dynamic" }`. Respons sama dengan `GET /qr`.
Token statis tetap disimpan saat beralih ke dinamis (QR cetak lama berlaku lagi bila kembali ke statis).

Error: 400 `QR_TYPE_INVALID`, 404 `NOT_FOUND`.

### POST /api/v1/queues/:queue_id/qr/replace
Mengganti token statis dengan kode baru. Token lama langsung ditolak saat join. Hanya untuk mode statis.

200: respons yang sama dengan `GET /qr` (`type` = `static`).
Error: 404 `NOT_FOUND`, 409 `QR_REPLACE_NOT_ALLOWED` (mode dinamis).

## Counter (owner)

Setiap antrian punya satu counter default (dibuat otomatis, `is_default` true) dan paling banyak 10 counter termasuk default. Semua endpoint butuh token dan hanya untuk owner antrian (bukan owner: 404 `NOT_FOUND`). Nama 1 sampai 30 karakter setelah dipangkas, unik per antrian.

### GET /api/v1/queues/:queue_id/counters
200:
```json
{ "data": { "action": "list_counters", "queue_id": 122, "counters": [
  { "id": 11, "name": "Counter 1", "is_default": true, "serving": { "ticket_id": 121, "ticket_number": 7 } },
  { "id": 12, "name": "Loket Cepat", "is_default": false, "serving": null } ] } }
```
Counter default dulu, lalu urut dibuat. `serving` null bila counter sedang tidak melayani.

### POST /api/v1/queues/:queue_id/counters
Body `{ "name": "Loket Cepat" }`. Nama kosong = nama otomatis `Counter N` (N terkecil yang belum dipakai, mulai dari 2). 201: `{ "data": { "id": 12, "name": "...", "is_default": false, "serving": null } }`.
Error: 400 `BAD_REQUEST` (nama lebih dari 30 karakter), 409 `DUPLICATE_COUNTER_NAME`, 409 `COUNTER_LIMIT`, 409 `QUEUE_CLOSED`.

### PUT /api/v1/queues/:queue_id/counters/:counter_id
Ganti nama (counter default boleh). Body `{ "name": "..." }`. 200: objek counter (tanpa info `serving`).
Error: 400, 404, 409 `DUPLICATE_COUNTER_NAME`, 409 `QUEUE_CLOSED`.

### DELETE /api/v1/queues/:queue_id/counters/:counter_id
Menghapus counter dalam satu transaksi; nomor yang sedang dilayani counter itu kembali ke posisi 1 urutan tunggu (`waiting`). 200: `{ "data": { "action": "delete_counter", "queue_id": 122, "counter_id": 12 } }`.
Error: 404, 409 `DEFAULT_COUNTER`, 409 `QUEUE_CLOSED`.

Setiap perubahan memancarkan event SSE `counter_updated` (lihat bagian Real-time).

## Riwayat antrian (owner)

Riwayat hanya memuat antrian yang **sudah ditutup**; antrian aktif tidak pernah muncul di sini (tetap di dashboard dan `GET /queues`). Data dibaca langsung dari tabel live. Semua endpoint butuh login, dan kepemilikan dicek di query: user yang bukan owner mendapat daftar kosong, dan 404 pada detail (sama dengan "antrian tidak ada", supaya keberadaan antrian orang lain tidak bocor).

Paginasi memakai kursor buram (`cursor`) dan `limit`. `next_cursor` bernilai `null` pada halaman terakhir. Kursor yang rusak: 400 `BAD_REQUEST`. `limit` bukan angka atau negatif: 400; 0 atau kosong memakai bawaan; nilai di atas batas dipangkas ke batas.

### GET /api/v1/history/queues?limit=&cursor=
Antrian milik user yang sudah ditutup, terbaru dulu (urut `closed_at` menurun, lalu `id` menurun). `limit` bawaan 20, maksimal 100.

200:
```json
{ "data": { "action": "list_history_queues",
  "queues": [ { "id": 54, "queue_name": "Loket", "ticket_prefix": "A",
    "opened_at": "2026-11-03T01:00:00Z", "closed_at": "2026-11-03T09:00:00Z",
    "total": 6, "completed": 2, "skipped": 1, "cancelled": 1, "deleted": 1, "unserved": 1 } ],
  "next_cursor": "1793700935123456.54" } }
```
Hitungan per hasil menjumlah ke `total`. `unserved` = nomor yang masih menunggu saat antrian ditutup (nomor yang sedang dilayani saat ditutup menjadi `completed`).

### GET /api/v1/history/queues/:queue_id?limit=&cursor=
Ringkasan satu antrian yang sudah ditutup dan satu halaman nomornya, urut nomor naik. `limit` bawaan 50, maksimal 200. Bukan milik owner, masih aktif, atau tidak ada: 404 `NOT_FOUND`.

200:
```json
{ "data": { "action": "history_queue",
  "queue": { "id": 54, "queue_name": "Loket", "...": "sama dengan di daftar" },
  "tickets": [ { "ticket_id": 56, "ticket_number": 3, "outcome": "completed", "skip_count": 0,
    "joined_at": "...", "called_at": "...", "completed_at": "...", "cancelled_at": null,
    "customer_name": "budi", "customer_email": "budi@mail.com",
    "counter_name": null, "note": "minta struk" } ],
  "next_cursor": null } }
```
`outcome`: `completed`, `skipped`, `cancelled`, `deleted`, atau `unserved`. `counter_name` hanya terisi bila antrian punya lebih dari satu counter. `note` adalah catatan owner (null bila tidak ada) dan tidak pernah terlihat oleh customer. Field yang bernilai null tetap ada di JSON.

### GET /api/v1/history/queues/:queue_id/export?format=csv|xlsx
Mengunduh semua nomor satu antrian yang sudah ditutup. Hanya owner; bukan owner, masih aktif, atau tidak ada: 404 `NOT_FOUND`. `format` wajib, selain `csv` atau `xlsx`: 400 `BAD_REQUEST`.

200: berkas (bukan JSON) dengan `Content-Disposition: attachment`, `Cache-Control: no-store`, `X-Content-Type-Options: nosniff`. Isi dialirkan; header unduhan baru dikirim saat byte pertama ditulis, jadi kegagalan sebelum itu masih dibalas JSON 500 biasa.

## Antrian (customer)

Info publik antrian untuk pemegang tiket diambil dengan `GET /queues/:queue_id` (lihat di atas; `PublicQueueResponse`).

### GET /api/v1/join/preview?t=<kode>
Info publik antrian dari kode join (login diperlukan). Backend mencari kode di `queue_settings` (`qr_current_code` untuk statis); kode yang sudah diganti atau kedaluwarsa ditolak.

200: `{ "data": { "action": "join_preview", "queue": PublicQueueResponse } }`
Error: 400 `BAD_REQUEST` (kode kosong), 404 `QR_INVALID`, 410 `QR_EXPIRED`.

### POST /api/v1/join
Customer bergabung lewat kode join. Body `{ "t": "K7M2QX" }`.

- Kode join dicari ulang di server (jangan percaya `queue_id` dari klien).
- Antrian belum punya tiket untuk user ini: tiket baru dibuat dengan `status` = `waiting`.
- Sudah punya tiket **aktif** (`waiting` atau `currently_served`): **tiket yang sudah ada dikembalikan** dengan 200 (bukan error, bukan tiket kedua). Pengecekan ini mendahului pengecekan verifikasi email, jadi scan ulang tidak berubah jadi galat.
- Antrian ditutup: 409 `QUEUE_CLOSED`.
- Tidak ada tiket aktif (tiket lama `completed`, `cancelled`, `skipped`, atau `deleted`): join membuat tiket baru dengan nomor baru (rejoin). Tiket lama tetap tidak berubah.

200: objek tiket langsung di dalam `data`.
```json
{ "data": { "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3, "status": "waiting", ... } }
```
Error: 400 `BAD_REQUEST` (kode kosong), 403 `EMAIL_NOT_VERIFIED` (hanya untuk nomor baru), 404 `QR_INVALID` / `NOT_FOUND`, 409 `QUEUE_CLOSED`,
410 `QR_EXPIRED`, 503 `RETRY_EXHAUSTED`.

Endpoint lama `POST /api/v1/queues/:queue_id/join` **dihapus** (join hanya lewat kode join).

### GET /api/v1/queues/:queue_id/status
Status antrian untuk owner atau pemegang tiket. Pengguna lain mendapat 404.

200:
```json
{
  "data": {
    "action": "queue_status",
    "queue": PublicQueueResponse,
    "my_ticket": TicketResponse | null,
    "current_number": 1 | null,
    "current_counter_name": "Loket Cepat" | null,
    "waiting_ahead": 3 | null,
    "previous_number": 0 | null,
    "next_number": 2 | null,
    "serving": [ { "counter_id": 11, "counter_name": null, "ticket_number": 1 } ]
  }
}
```
- `current_number`: nomor yang sedang dilayani (tiket `currently_served`); `current_counter_name` = nama counter-nya.
- `serving`: satu elemen per counter yang sedang melayani nomor (`counter_name` null untuk counter default).
- `previous_number`: nomor sebelumnya.
- `next_number`: nomor tiket `waiting` berikutnya.
- `waiting_ahead`: jumlah tiket `waiting` di depan milik requester. Null bila requester bukan pemegang tiket `waiting`.

## Layar publik (fitur 4.5)

Layar publik menampilkan satu kartu per counter, nomor berikutnya, dan nomor dilewati di monitor ruang tunggu, tanpa login. Satu-satunya kredensial adalah **token acak di URL** (`/display/<token>`, 32 karakter base64url dari 24 byte), bukan `queue_id`, supaya antrian tidak bisa ditelusuri. Konfigurasi hanya untuk owner antrian dan tidak butuh langganan aktif.

### GET /api/v1/queues/:queue_id/display
Butuh token (owner). Konfigurasi layar. 404 bila layar belum diaktifkan atau bukan owner antrian.

200:
```json
{ "data": { "queue_id": 5, "token": "Z8x7LgV0pgl1DEZ7EIuOIytZ11ndM5th", "upcoming_count": 5, "skipped_count": 0,
            "created_at": "...", "rotated_at": null } }
```
Klien membentuk URL layar dari origin web + `/display/` + `token`.

### PUT /api/v1/queues/:queue_id/display
Butuh token (owner). Mengaktifkan layar (token baru dibuat) atau mengubah jumlahnya (token tetap).

Body (semua opsional): `{ "upcoming_count": 5, "skipped_count": 0 }`. Yang tidak dikirim: nilai sekarang bila layar sudah ada, kalau tidak 5 dan 0. `0` adalah nilai sah dan menyembunyikan bagian itu di layar.

200: objek konfigurasi seperti di atas. Error: 400 `BAD_REQUEST` (di luar 0 sampai 10, atau badan rusak), 404 (bukan owner).

### POST /api/v1/queues/:queue_id/display/rotate
Butuh token (owner). Mengganti token: tautan lama langsung 404, termasuk layar yang sedang terbuka (mereka menerima sinyal dan menampilkan "Tautan layar tidak berlaku"). 200: konfigurasi dengan token baru. 404 bila belum diaktifkan atau bukan owner.

### DELETE /api/v1/queues/:queue_id/display
Butuh token (owner). Mematikan layar. Idempoten. 204.

### GET /api/v1/display/:token
**Tanpa token login.** Snapshot untuk layar. Di-cache di server 1 detik per layar dan dibatalkan tiap ada event antrian, jadi pembacaan setelah sinyal selalu segar.

200:
```json
{ "data": { "queue_name": "Loket A", "queue_desc": "Pelayanan umum", "status": "active", "multi_counter": true,
            "counters": [ { "id": 11, "name": "Counter 1", "serving": "A-007" },
                          { "id": 12, "name": "Loket Cepat", "serving": null } ],
            "upcoming": ["A-008", "A-009"], "skipped": [], "generated_at": "..." } }
```
- `counters`: **semua** counter antrian, counter default dulu lalu urut dibuat, termasuk yang sedang kosong (`serving` null). Klien menggambar satu kartu per elemen.
- `multi_counter` false (hanya counter default): klien tidak menampilkan nama counter.
- Nomor sudah berbentuk teks siap tampil (`A-007`, awalan `ticket_prefix`, tiga digit). `upcoming` = tiket `waiting` terdepan sebanyak `upcoming_count`; `skipped` = tiket `skipped` terbaru sebanyak `skipped_count`.
- `status` `closed`: antrian ditutup (tetap 200, bukan 404).
- **Respons ini publik.** Tidak pernah memuat id tiket, id user, `owner_id`, nama customer, `sort_key`, atau catatan owner. Tes kontrak mengunci daftar kunci.

404 `NOT_FOUND` yang sama untuk token salah bentuk, tidak dikenal, sudah diganti, atau layar dimatikan. Header `Cache-Control: no-store`.

### GET /api/v1/display/:token/event
**Tanpa token login.** SSE untuk layar. Berbeda dari `/queues/:id/event`: layar masuk room terpisah yang **hanya** menerima sinyal tanpa data, `{"type":"display_changed","queue_id":5,"ts":"..."}`, yang berarti "ambil ulang snapshot". Payload tiket (yang memuat id user) tidak pernah dikirim ke sini. Satu aksi bisa memancarkan lebih dari satu sinyal (Skip memancarkan dua); klien menggabungkannya. Sinyal tidak dikirim untuk `queue_qr_updated`.

Heartbeat `: ping` tiap 25 detik seperti stream biasa. Error: 404 (token tidak berlaku), 429 `TOO_MANY_DISPLAYS` (lebih dari 20 sambungan layar bersamaan untuk satu antrian), 503.

nginx: zona laju `display_zone` (20 permintaan/detik per IP, burst 40) dan log akses yang menyamarkan token (`/api/v1/display/***`).

## Langganan owner (BorderPay)

Dilayani backend, pembayaran lewat payment-service dan BorderPay.

### POST /api/v1/owners/subscription/checkout
Butuh token. Body `{ "method": "qris", "return_url": "https://..." }` (hanya QRIS yang didukung; `return_url` opsional, halaman tujuan setelah membayar, harus https). Membuat pesanan pembayaran satu bulan lewat payment-service dan BorderPay. Harga ditentukan server dan saat ini tetap 15000 (`SubscriptionPriceIDR` di `backend/internal/config/config.go` tidak dibaca dari environment; variabel `SUBSCRIPTION_PRICE_IDR` tidak berpengaruh). Langganan **tidak** bertambah di sini: `owners.subscription_expires_at` diperpanjang 30 hari oleh webhook BorderPay `payment.paid` setelah pembayaran lunas. Calon owner otomatis menjadi owner pada pembayaran pertama yang lunas.

Respons 200:
```json
{ "data": { "reference_id": "SUB-7-1794539540-a1b2c3d4e5f6", "pay_url": "https://borderpay.id/pay/...", "expires_at": "2026-11-13T04:12:20Z", "amount": 15000 } }
```
Galat: 400 `INVALID_PAYMENT_METHOD`, 403 `EMAIL_NOT_VERIFIED`, 502 `PAYMENT_REJECTED`, 503 `PAYMENT_UNAVAILABLE`.
Error: 401 `UNAUTHORIZED` (tanpa token atau token tidak valid), 500 `INTERNAL_ERROR` (galat database; tidak ada yang berubah). Lewat nginx, path ini masuk ke backend (`location /api/`).

## Webhook pembayaran (BorderPay)

### POST /api/v1/webhooks/borderpay

Publik, tanpa JWT. Dipanggil BorderPay saat transaksi QRIS lunas. Header `x-borderpay-token` harus sama dengan `BORDERPAY_WEBHOOK_TOKEN` (token kosong = semua request ditolak). Hanya payload dengan `data.status` = `paid` yang mengubah data; status lain dijawab 200 tanpa efek. Pesanan dicocokkan dengan `reference_id`. Idempoten: pengiriman ulang untuk pesanan yang sama tidak menerapkan pembayaran dua kali.

| Status | Arti |
|---|---|
| 200 | Diterapkan (`{"status":"ok"}`), sudah pernah diterapkan, status bukan `paid`, atau `reference_id` tidak dikenal (diabaikan) |
| 400 | Body terlalu besar atau bukan JSON, atau `reference_id` kosong |
| 401 | `x-borderpay-token` tidak cocok dengan `BORDERPAY_WEBHOOK_TOKEN` |
| 409 | Nominal webhook tidak sama dengan pesanan; pembayaran tidak diterapkan |
| 500 | Penyimpanan ke database gagal; BorderPay mengirim ulang |

Lewat nginx, path ini masuk ke payment-service dan tidak dibatasi rate limit.

## Objek

### QueueResponse
```json
{ "id": 122, "owner_id": 1, "queue_name": "Loket A", "queue_desc": "Pelayanan umum", "ticket_prefix": "A",
  "status": "active", "created_at": "...", "closed_at": null }
```
`status`: `active` atau `closed`. `queue_desc` bisa null.

### PublicQueueResponse
Sama seperti QueueResponse tanpa `owner_id`.

### TicketResponse
```json
{ "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3,
  "status": "waiting", "skip_count": 0, "last_notified_at": null,
  "created_at": "...", "completed_at": null, "cancelled_at": null,
  "called_counter_id": null, "called_counter_name": null, "called_at": null }
```
`called_counter_id`/`called_at` terisi saat nomor dipanggil (dikosongkan bila dikembalikan ke `waiting`). `called_counter_name` pada `TicketResponse` saat ini selalu null; nama counter ada di `TicketEvent` (hanya untuk counter non-default) dan di `current_counter_name`/`serving`.
`status`: `waiting`, `currently_served`, `completed`, `cancelled`, `skipped`, `deleted`.
`ticket_number` adalah nomor yang dilihat customer. `sort_key` adalah kunci urut internal.

## Real-time (SSE)

### GET /api/v1/queues/:queue_id/event
Stream Server-Sent Events. Butuh token di header `Authorization` (karena itu klien memakai `fetch`
stream, bukan `EventSource`). Hanya owner atau pemegang tiket. Selain itu 404.

Respons header: `Content-Type: text/event-stream`, `X-Accel-Buffering: no`.

Urutan awal:
```
: connected

```
Setiap ~25 detik backend mengirim heartbeat:
```
: ping

```

Setiap event berbentuk:
```
data: {"type":"ticket_next","queue_id":122,"payload":{...},"ts":"2026-10-17T09:29:59.659Z"}

```

| `type` | Dikirim saat | `payload` |
|---|---|---|
| `ticket_joined` | Customer berhasil join (tiket baru) | TicketEvent |
| `ticket_next` | Owner menekan Next | TicketEvent dengan `status` = `currently_served` |
| `ticket_skipped` | Owner menekan Skip | TicketEvent dengan `status` akhir tiket (`skipped`, atau `waiting` bila dikembalikan); disusul `ticket_next` untuk nomor yang dipanggil |
| `ticket_cancelled` | Customer membatalkan nomornya | TicketEvent dengan `status` = `cancelled`; disusul `ticket_next` bila nomor berikutnya naik |
| `ticket_removed` | Owner menghapus nomor customer | TicketEvent dengan `status` = `deleted`; disusul `ticket_next` bila nomor berikutnya naik |
| `queue_updated` | Nama atau deskripsi antrian diubah | `{ "queue_id": 122 }` |
| `queue_closed` | Antrian ditutup | `{ "queue_id": 122 }` |
| `queue_qr_updated` | Mode QR diubah atau token statis diganti | `{ "queue_id": 122 }` |
| `counter_updated` | Counter dibuat, diganti nama, atau dihapus | `{ "queue_id": 122, "counter_id": 12, "action": "created" \| "renamed" \| "deleted" }` |

Catatan: `queue_updated`, `queue_closed`, `queue_qr_updated`, dan `counter_updated` memakai payload objek biasa, bukan TicketEvent. Klien hanya perlu `queue_id` untuk mengambil ulang status.

### TicketEvent
Sama seperti TicketResponse, tetapi tanpa `user_id`, `sort_key`, dan `last_notified_at`; `called_counter_name` terisi untuk counter non-default. Alasannya: penerima event bisa
customer lain di antrian yang sama, jadi identitas dan urutan internal tidak ikut dikirim.

Klien harus mengambil ulang daftar tiket (atau status) setelah menerima event, dan setelah koneksi pulih dari putus.

## Catatan kompatibilitas

- Join ulang mengembalikan 200 dengan tiket yang sama. Klien lama yang masih menangani 409 `ALREADY_JOINED` tetap aman,
  karena jalur itu tidak lagi dipanggil.
- `POST /join` dan `POST /next` mengembalikan objek tiket langsung (tanpa `action`). Endpoint lain memakai `action`.
- Endpoint `GET /api/v1/queues/:id/info` dihapus; pemegang tiket memakai `GET /api/v1/queues/:id`.
- Join berbasis id (`POST /api/v1/queues/:id/join`) dihapus. Klien harus memakai token dari QR
  (`GET /join/preview?t=...` lalu `POST /join {t}`). Token statis yang sudah diganti ditolak dengan 404 `QR_INVALID`.