# Plan: sisip customer di posisi tertentu (US-10 / 4.12)

Dokumen kerja. Fitur P3 backlog. Plan ini mencakup backend, frontend, dan tes.

## 1. Latar belakang

Owner perlu memasukkan customer ke posisi tertentu di antrian, untuk melewati urutan normal (mis. pengecualian walk-in). Spesifikasi 4.12:

- Sisip di posisi 1 saat ada yang sedang dilayani: nomor baru menjadi "berikutnya", bukan "sedang dilayani". Nomor yang sedang dilayani tidak terganggu.
- Posisi melebihi panjang antrian: masuk di paling belakang, tanpa error.
- Dua sisip di posisi yang sama bersamaan: diproses sesuai urutan masuk; yang kedua tepat setelah yang pertama.
- Nomor di belakang posisi sisip bergeser satu posisi ke belakang.

Keputusan yang sudah dikunci:

| Topik | Keputusan |
|---|---|
| Siapa yang disisipkan | **Akun terdaftar.** Owner mencari akun lewat email atau username. Tiket wajib punya `user_id`, dan customer harus login, jadi tidak ada tamu tanpa akun. Tanpa perubahan skema. |
| Akun ambigu | Username tidak unik di skema. Bila pencarian mengembalikan lebih dari satu akun, sisip ditolak dengan `AMBIGUOUS_ACCOUNT` dan owner diminta memakai email. |
| Akun sudah punya tiket aktif | Ditolak: 409 `ALREADY_JOINED`. Owner tidak boleh menyisipkan ganda. Tiket lama tidak diubah. |
| Definisi posisi | 1-based, dihitung dari tiket `waiting` (urutan tunggu). Posisi 1 = "berikutnya". Tiket `currently_served` tidak ikut dihitung. |

## 2. Cakupan

Termasuk: endpoint sisip, pencarian akun untuk owner, perhitungan `sort_key` di antara dua tiket waiting, event SSE, UI owner (form di detail antrian), tes, dan dokumen.

Tidak termasuk: push ke customer yang disisipkan (sama seperti remove, ditunda), counter, dan sisip ke antrian yang sudah ditutup (ditolak, 409 `QUEUE_CLOSED`).

## 3. Backend

### 3.1 Endpoint

`POST /api/v1/queues/:queue_id/insert`, owner saja. Body:

```json
{ "identifier": "email atau username", "position": 2 }
```

Validasi: `position` ≥ 1. Di luar itu 400 `BAD_REQUEST`.

Respons 201: objek tiket baru (`status` = `waiting`), dengan `ticket_number` berikutnya seperti join biasa.

Error:
- 404 `NOT_FOUND`: bukan owner, atau antrian tidak ada.
- 404 `NOT_FOUND`: akun tidak ditemukan (dengan pesan yang sama untuk email dan username, tidak membedakan keduanya).
- 409 `AMBIGUOUS_ACCOUNT`: lebih dari satu akun cocok dengan username.
- 409 `ALREADY_JOINED`: akun sudah punya tiket aktif di antrian ini.
- 409 `QUEUE_CLOSED`.

### 3.2 Pencarian akun

`UserRepository.FindByLogin(identifier)` sudah ada dan mengembalikan paling banyak 10 baris, email dicocokkan lebih dulu. Logika di service:

- Nol baris: `ErrNotFound`.
- Satu baris: pakai akun itu.
- Lebih dari satu: `ErrAmbiguousAccount` (baru, `apperror`). Owner diminta memakai email.

Akun Google-only (tanpa password) tetap boleh disisipkan, karena tiket hanya butuh `user_id`.

`TicketService` belum punya akses ke user repository. Injeksi interface kecil `AccountFinder` (satu method `FindByLogin`), dipasang lewat `WithAccountFinder`, sama seperti pola `WithQueueGuard`. Di `cmd/server/main.go` diisi dengan `userRepo`.

### 3.3 Perhitungan sort_key

Di dalam satu transaksi (`s.tx.WithTx`):

1. Ambil `sort_key` tiket waiting dengan `WaitingSortKeys(queueID, limit = position)`, urut kecil ke besar. Ini memakai ulang method yang sudah ada.
2. Tentukan batas:
   - `lower` = `keys[position-2]` bila `position ≥ 2`, selain itu `0`.
   - `upper` = `keys[position-1]` bila ada, selain itu tidak ada (sisip di belakang).
3. Bila tidak ada `upper`: `sortKey = MaxWaitingSortKey + SortKeyGap` (sama dengan join).
4. Bila ada `upper`: `mid = lower + (upper-lower)/2`. Bila `mid > lower` dan `SortKeyTaken(mid)` false, pakai `mid`.
5. Bila jarak habis atau bentrok: `RebalanceWaiting(queueID, SortKeyGap)`, lalu ulangi langkah 1–4. Maksimal dua percobaan, sama seperti `requeueSortKey`. Gagal: `ErrRetryExhausted` (503).

Ini memakai ulang `requeueSortKey` sebagai pola. Sebaiknya diekstrak ke helper `sortKeyAt(ctx, tx, queueID, position)` supaya requeue dan sisip memakai satu implementasi. Perubahan ini tidak mengubah perilaku requeue; tes skip yang ada harus tetap hijau.

Insert ke tiket yang sudah dibuat: `Insert(queueID, userID, sortKey)` yang sudah ada. Ticket number tetap `MAX+1`, jadi nomor tidak bergeser; hanya urutan tunggu yang berubah.

### 3.4 Konkurensi

- Dua sisip di posisi yang sama: keduanya membaca `keys` yang sama. Yang pertama menulis `mid`. Yang kedua gagal di `UNIQUE (queue_id, sort_key)` (`ErrDuplicate`) atau melihat `SortKeyTaken` true, lalu mengulang langkah 1–4 dengan urutan yang sudah berubah. Hasilnya tepat setelah yang pertama, sesuai 4.12.
- Sisip bersamaan dengan Next: Next mengunci `currently_served` lalu `waiting` teratas (urutan kunci yang sudah berlaku). Sisip tidak menyentuh `currently_served`, jadi tidak ada deadlock baru. Sisip hanya mengunci lewat `Insert` dan `RebalanceWaiting`, dan keduanya tidak mengunci `currently_served`.
- Race dengan join akun yang sama: `UNIQUE (queue_id, user_id)` berbasis index parsial. Yang kalah menerima `ErrDuplicate`, lalu `JoinQueue`/`Insert` mengecek ulang `HasTicket` dan mengembalikan `ALREADY_JOINED`.

### 3.5 Event

`EventTicketJoined` dengan `dto.NewTicketEvent(ticket)`. Owner dan papan customer memuat ulang lewat event yang sudah ada (`ticket_joined` sudah didengarkan `use-queue-board` dan `use-ticket-queue`). Tidak perlu konstanta baru.

### 3.6 Posisi 1 saat ada yang sedang dilayani

Sisip di posisi 1 membuat tiket menjadi `waiting` dengan sort_key terkecil. Tiket yang sedang dilayani tidak disentuh, jadi nomor baru menjadi "berikutnya" sesuai 4.12. Tidak perlu logika khusus.

### 3.7 Antrian kosong dan posisi melebihi panjang

- Antrian tanpa waiting, posisi berapa pun: `keys` kosong, sisip di belakang, `sortKey = MaxWaitingSortKey + gap`.
- Posisi > jumlah waiting + 1: `upper` tidak ada, masuk di paling belakang. Tidak error.

## 4. Frontend

- `src/lib/tickets.ts`: `insertTicket(queueId, identifier, position)` → `POST .../insert`.
- Di detail antrian (`QueueDetail.tsx`), tombol "Sisip customer" dekat daftar tiket. Modal berisi input identifier dan input posisi (angka, default = panjang waiting + 1). Simpan memanggil API.
- Error ditampilkan di modal: `AMBIGUOUS_ACCOUNT` menyarankan memakai email; `ALREADY_JOINED` dan `QUEUE_CLOSED` ditampilkan apa adanya.
- Tombol nonaktif bila antrian ditutup.
- Validasi di klien: identifier tidak kosong, posisi bilangan bulat ≥ 1.
- Hook: aksi `insert` di `use-queue-board` (pola sama dengan `remove`), memuat ulang papan sendiri.
- Tes unit: fungsi murni `validateInsertInput(identifier, position)` di `lib/tickets.ts` (identifier kosong, posisi 0, posisi pecahan ditolak).

## 5. Tes

**Service (fake):**
- Sisip di tengah: urutan `waiting` berubah sesuai posisi; tiket di belakang geser satu posisi.
- Posisi 1 saat ada yang dilayani: tiket baru di `waiting` teratas, `currently_served` tidak berubah.
- Posisi melebihi panjang: masuk paling belakang, tanpa error.
- Antrian kosong: posisi berapa pun menghasilkan tiket pertama.
- Akun tidak ditemukan: `ErrNotFound`.
- Akun ambigu: `ErrAmbiguousAccount`.
- Akun sudah punya tiket aktif: `ErrAlreadyJoined`.
- Akun yang sudah `completed`: boleh disisipkan (nomor baru).
- Non-owner: `ErrNotFound`. Antrian ditutup: `ErrQueueClosed`.
- Dua sisip di posisi sama berurutan: yang kedua tepat setelah yang pertama.
- Ruang sort_key habis: rebalance terpanggil dan sisip tetap berhasil.
- Regresi: semua tes skip (`requeueSortKey`) tetap hijau setelah diekstrak.

**Repository (fake dan integrasi):** `WaitingSortKeys` dengan limit; `SortKeyTaken`; `RebalanceWaiting` menjaga urutan relatif.

**Integrasi Postgres (butuh `DATABASE_URL`):** dua sisip paralel di posisi sama: keduanya tersimpan, tanpa error selain `ALREADY_JOINED` untuk akun yang sama, dan urutan akhir konsisten. Sisip paralel dengan Next: tidak ada deadlock.

**Handler:** pemetaan 201, 400, 404, 409 (ambigu, sudah join, ditutup).

**Playwright (`ts-18-sisip.spec.ts`):**
- Owner menyisipkan akun di posisi 1 saat ada yang dilayani: dashboard menampilkan akun itu sebagai "berikutnya".
- Owner menyisipkan dengan email yang tidak terdaftar: pesan "tidak ditemukan" tampil.
- Owner menyisipkan akun yang sudah ikut antrian: pesan "sudah ikut antrian" tampil.

## 6. Urutan kerja

1. `apperror.ErrAmbiguousAccount`, dan ekstrak helper `sortKeyAt` dari `requeueSortKey` (tes skip harus tetap hijau).
2. Repository: tidak ada method baru selain yang sudah ada. Tes tambahan untuk `WaitingSortKeys` dengan limit.
3. Service: `InsertTicket`, `AccountFinder`, `WithAccountFinder`, dan wiring di `main.go`.
4. Handler, route, pemetaan error, dan tes handler.
5. Frontend: API, modal, hook, dan tes unit validasi.
6. Playwright `ts-18`.
7. Dokumen: `api-contract.md` (endpoint dan kode error baru), `FLOW.md`.

## 7. Verifikasi

1. `cd backend && go vet ./... && go vet -tags integration ./internal/test/integration_test/ && go test ./...`
2. `cd frontend && npx tsc --noEmit && npm run test:unit`
3. Rebuild backend dan frontend, seed ulang, lalu Playwright penuh. Pastikan tes skip tetap hijau setelah ekstraksi helper.
4. Suite integrasi dengan `DATABASE_URL`, dijalankan di lingkungan Anda.

## 8. Risiko

| Risiko | Mitigasi |
|---|---|
| Username ganda menyisipkan akun yang salah | Ditolak dengan `AMBIGUOUS_ACCOUNT`; email selalu cocok unik |
| Sisip paralel di posisi sama menghasilkan urutan acak | Urutan tiba diproses lewat UNIQUE dan retry; yang kedua tepat setelah yang pertama |
| Ekstraksi helper mengubah perilaku skip | Tes skip yang ada harus hijau sebelum lanjut ke sisip |
| Ruang sort_key habis di antrian lama | `RebalanceWaiting` dipanggil otomatis, sama seperti requeue |
| Owner menyisipkan akun yang sebenarnya sudah ikut antrian | `ALREADY_JOINED` sebelum insert; race ditangani UNIQUE parsial |

## 9. Catatan

- Plan ini tidak menambah push. Customer yang disisipkan tidak mendapat notifikasi, sama seperti remove.
- Tidak ada perubahan skema.
