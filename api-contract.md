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
| Autentikasi | `Authorization: Bearer <token>` (JWT HS256, berlaku 24 jam, klaim `sub` = user id) |
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
| 404 | `NOT_FOUND` | Antrian tidak ada, atau requester tidak punya hak melihatnya |
| 409 | `CONFLICT` | Email atau username sudah terdaftar (register) |
| 409 | `QUEUE_CLOSED` | Antrian sudah ditutup (join) |
| 409 | `QUEUE_EMPTY` | Tidak ada tiket yang menunggu (next) |
| 409 | `ALREADY_JOINED` | Dicadangkan. Join ulang saat ini mengembalikan tiket yang sudah ada dengan 200 |
| 409 | `QR_REPLACE_NOT_ALLOWED` | Penggantian kode QR hanya untuk mode statis |
| 400 | `QR_TYPE_INVALID` | `type` QR bukan `static`/`dynamic` |
| 404 | `QR_INVALID` | Token join tidak valid, sudah diganti, atau antrian tidak ada |
| 410 | `QR_EXPIRED` | Token join dinamis sudah kedaluwarsa |
| 500 | `INTERNAL_ERROR` | Kesalahan server |
| 503 | `QR_UNAVAILABLE` | qr-service tidak bisa dihubungi/timeout |
| 503 | `RETRY_EXHAUSTED` | Join gagal setelah beberapa percobaan karena banyak permintaan bersamaan |

Catatan: 404 dipakai juga untuk "bukan milik kamu". Ini disengaja agar keberadaan antrian orang lain tidak bocor.

## Autentikasi

### POST /api/v1/users/register
Tanpa token.

Request:
```json
{ "email": "owner@contoh.com", "username": "owner1", "password": "rahasia123" }
```
Semua field wajib. `email` dan `username` tidak boleh mengandung tag HTML.

200:
```json
{ "data": { "token": "eyJhbGciOi..." } }
```
Error: 400 `BAD_REQUEST`, 409 `CONFLICT`, 500 `INTERNAL_ERROR`.

### POST /api/v1/users/login
Tanpa token. Menerima email atau username lewat `identifier`. Field `email` tetap diterima sebagai alias.

Request:
```json
{ "identifier": "owner@contoh.com", "password": "rahasia123" }
```

200: `{ "data": { "token": "..." } }`
Error: 400 `BAD_REQUEST` (identifier dan email kosong), 401 `UNAUTHORIZED` (kredensial salah; pesan tidak membedakan mana yang salah).

### GET /api/v1/users/me
Butuh token.

200:
```json
{ "data": { "id": 1, "email": "owner@contoh.com", "username": "owner1", "is_owner": true } }
```
`is_owner` = true bila user punya baris di tabel `owners`.

### GET /api/v1/users/me/tickets
Butuh token. Tiket milik user di semua antrian.

200:
```json
{
  "data": {
    "action": "list_my_tickets",
    "tickets": [
      {
        "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3,
        "status": "waiting", "skip_count": 0, "last_notified_at": null,
        "created_at": "2026-10-17T16:29:57+07:00", "completed_at": null, "cancelled_at": null,
        "queue_name": "Loket A", "queue_status": "active", "current_number": 1
      }
    ]
  }
}
```
`current_number` = `ticket_number` yang sedang dilayani di antrian itu, atau null.

## Antrian (owner)

Semua endpoint di bagian ini butuh token. Kecuali `GET /queues/:id/info` dan `GET /queues/:id/status`,
hanya owner antrian yang boleh memakainya.

### POST /api/v1/queues
Membuat antrian. Butuh langganan owner aktif.

Request:
```json
{ "queue_name": "Loket A", "queue_description": "Pelayanan umum" }
```
`queue_name` wajib. `queue_description` boleh kosong.

201:
```json
{ "data": { "action": "create_queue", "queue": { ...QueueResponse }esponse } } }
```
Error: 400 `BAD_REQUEST`, 402 `SUBSCRIPTION_INACTIVE`, 500.

### GET /api/v1/queues
Daftar antrian milik owner. Bila langganan tidak aktif, hanya antrian yang masih aktif yang dikembalikan (tidak ada 402).

200: `{ "data": { "action": "list_queues", "queues": [ ...QueueResponse ] } }`

### GET /api/v1/queues/:queue_id
Detail antrian beserta izin requester. Hanya owner.

200: `{ "data": { "action": "get_queue", "queue": {...} } }`
Error: 400, 404 `NOT_FOUND`.

### PUT /api/v1/queues/:queue_id
Ubah nama dan deskripsi. Butuh langganan aktif dan hak owner.

Request: `{ "queue_name": "Loket B", "queue_description": "..." }`
200: `{ "data": { "action": "update_queue", ... } }`
Error: 400, 402 `SUBSCRIPTION_INACTIVE`, 404.

### POST /api/v1/queues/:queue_id/close
Menutup antrian. Tidak bisa dibuka lagi. Customer tidak bisa join setelah ditutup.

200: `{ "data": { "action": "close", ... } }`
Error: 400, 404.

### POST /api/v1/queues/:queue_id/next
Memanggil tiket `waiting` pertama dan menandainya `completed`. Hanya owner.

200: objek tiket langsung di dalam `data`, dengan `status` = `"completed"`.
```json
{ "data": { "id": 121, "queue_id": 122, "ticket_number": 2, "status": "completed", "completed_at": "...", ... } }
```
Error: 404 `NOT_FOUND` (bukan owner), 409 `QUEUE_EMPTY` (tidak ada yang menunggu).

### POST /api/v1/queues/:queue_id/skip
Melewati nomor yang sedang dilayani (tiket `completed` dengan `called_at` terbaru) lalu langsung memanggil tiket `waiting` pertama, dalam satu transaksi. Hanya owner. Tiket yang dilewati menjadi `skipped`, `skip_count` +1, `completed_at` dikosongkan (`called_at` tetap). Customer yang dilewati belum bisa kembali.

200: objek tiket yang baru dipanggil (sama seperti Next), `status` = `"completed"`.
Error: 404 `NOT_FOUND` (bukan owner), 409 `NOTHING_TO_SKIP` (belum ada nomor yang dilayani), 409 `QUEUE_EMPTY` (tidak ada yang menunggu; tidak ada perubahan, nomor tetap dilayani).

### GET /api/v1/queues/:queue_id/tickets
Semua tiket antrian, urut `sort_key`. Hanya owner. Maksimal 500 baris.

Query opsional: `status` (`waiting`, `completed`, `cancelled`, `skipped`), boleh diulang atau dipisah koma.

200:
```json
{ "data": { "action": "list_tickets", "queue_id": 122, "tickets": [ ...TicketResponse ] } }
```
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
    "join_url": "https://qms.example.com/join?t=v1...",
    "expires_at": null
  }
}
```
`type` = `static` | `dynamic`. `expires_at` hanya untuk dinamis (detik Unix).

Error: 404 `NOT_FOUND` (bukan owner), 503 `QR_UNAVAILABLE`.

### PUT /api/v1/queues/:queue_id/qr/type
Mengubah mode QR. Body `{ "type": "static" | "dynamic" }`. Respons sama dengan `GET /qr`.
Token statis tetap disimpan saat beralih ke dinamis (QR cetak lama berlaku lagi bila kembali ke statis).

Error: 400 `QR_TYPE_INVALID`, 404 `NOT_FOUND`, 503 `QR_UNAVAILABLE`.

### POST /api/v1/queues/:queue_id/qr/replace
Mengganti token statis dengan kode baru. Token lama langsung ditolak saat join. Hanya untuk mode statis.

200: respons yang sama dengan `GET /qr` (`type` = `static`).
Error: 404 `NOT_FOUND`, 409 `QR_REPLACE_NOT_ALLOWED` (mode dinamis), 503 `QR_UNAVAILABLE`.

## Antrian (customer)

### GET /api/v1/queues/:queue_id/info
Info publik antrian untuk pengguna yang sudah login. Tanpa daftar tiket.

200: `{ "data": { "action": "queue_info", "queue": PublicQueueResponse } }`
Error: 404 `NOT_FOUND`.

### GET /api/v1/join/preview?t=<token>
Info publik antrian dari token QR (login diperlukan). Backend memverifikasi token ke qr-service;
untuk token statis, token juga harus sama dengan `queue_settings.qr_current_code` (penggantian mencabut token lama).

200: `{ "data": { "action": "join_preview", "queue": PublicQueueResponse } }`
Error: 400 `BAD_REQUEST` (token kosong), 404 `QR_INVALID`, 410 `QR_EXPIRED`, 503 `QR_UNAVAILABLE`.

### POST /api/v1/join
Customer bergabung lewat token QR. Body `{ "t": "v1..." }`.

- Token diverifikasi ulang (jangan percaya `queue_id` dari klien).
- Antrian belum punya tiket untuk user ini: tiket baru dibuat dengan `status` = `waiting`.
- Sudah punya tiket: **tiket yang sudah ada dikembalikan** (bukan error, bukan tiket kedua). Ini juga berlaku setelah tiket itu `completed`.
- Antrian ditutup: 409 `QUEUE_CLOSED`.

200: objek tiket langsung di dalam `data`.
```json
{ "data": { "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3, "status": "waiting", ... } }
```
Error: 400 `BAD_REQUEST` (token kosong), 404 `QR_INVALID` / `NOT_FOUND`, 409 `QUEUE_CLOSED`,
410 `QR_EXPIRED`, 503 `QR_UNAVAILABLE` / `RETRY_EXHAUSTED`.

Endpoint lama `POST /api/v1/queues/:queue_id/join` **dihapus** (join hanya lewat token QR).

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
    "waiting_ahead": 3 | null,
    "previous_number": 0 | null,
    "next_number": 2 | null
  }
}
```
- `current_number`: nomor yang sedang dilayani (tiket `completed` terbaru).
- `previous_number`: nomor sebelumnya.
- `next_number`: nomor tiket `waiting` berikutnya.
- `waiting_ahead`: jumlah tiket `waiting` di depan milik requester. Null bila requester bukan pemegang tiket `waiting`.

## Objek

### QueueResponse
```json
{ "id": 122, "owner_id": 1, "queue_name": "Loket A", "queue_desc": "Pelayanan umum",
  "status": "active", "created_at": "...", "closed_at": null }
```
`status`: `active` atau `closed`. `queue_desc` bisa null.

### PublicQueueResponse
Sama seperti QueueResponse tanpa `owner_id`.

### TicketResponse
```json
{ "id": 121, "queue_id": 122, "ticket_number": 2, "sort_key": 131072, "user_id": 3,
  "status": "waiting", "skip_count": 0, "last_notified_at": null,
  "created_at": "...", "completed_at": null, "cancelled_at": null }
```
`status`: `waiting`, `completed`, `cancelled`, `skipped`.
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
| `ticket_next` | Owner menekan Next | TicketEvent dengan `status` = `completed` |
| `ticket_skipped` | Owner menekan Skip | TicketEvent dengan `status` = `skipped`; disusul `ticket_next` untuk nomor yang dipanggil |
| `queue_updated` | Nama atau deskripsi antrian diubah | `{ "queue_id": 122 }` |
| `queue_closed` | Antrian ditutup | `{ "queue_id": 122 }` |
| `queue_qr_updated` | Mode QR diubah atau token statis diganti | `{ "queue_id": 122 }` |

Catatan: `queue_updated` dan `queue_closed` memakai payload objek biasa, bukan TicketEvent. Klien hanya perlu `queue_id` untuk mengambil ulang status.

### TicketEvent
Sama seperti TicketResponse, tetapi tanpa `user_id`, `sort_key`, dan `last_notified_at`. Alasannya: penerima event bisa
customer lain di antrian yang sama, jadi identitas dan urutan internal tidak ikut dikirim.

Klien harus mengambil ulang daftar tiket (atau status) setelah menerima event, dan setelah koneksi pulih dari putus.

## Catatan kompatibilitas

- Join ulang mengembalikan 200 dengan tiket yang sama. Klien lama yang masih menangani 409 `ALREADY_JOINED` tetap aman,
  karena jalur itu tidak lagi dipanggil.
- `POST /join` dan `POST /next` mengembalikan objek tiket langsung (tanpa `action`). Endpoint lain memakai `action`.
- Join berbasis id (`POST /api/v1/queues/:id/join`) dihapus. Klien harus memakai token dari QR
  (`GET /join/preview?t=...` lalu `POST /join {t}`). Token statis yang sudah diganti ditolak dengan 404 `QR_INVALID`.
