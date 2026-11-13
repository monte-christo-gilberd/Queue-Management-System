# Migrasi database

Skema PostgreSQL Queue Management System, dipecah per domain. Folder ini hanya untuk **database kosong**.
Database yang sudah berjalan memakai [`../scripts/apply_schema.sql`](../scripts/apply_schema.sql): berkas itu idempoten dan memuat semua perubahan skema belakangan ini (kolom dan constraint tabel, `notification_outbox` dengan `queue_id`, `ticket_id` nullable, `uq_outbox_kind_queue` dan `kind` `queue-closed`, serta fungsi dan trigger `tickets_notify`, `tickets_email_outbox`, `queues_email_outbox`). Jalankan dengan `psql -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql` (perintah lengkap dev dan prod ada di kepala berkasnya), lalu restart `backend` dan `notify-service`. Backup dulu di prod.

## Urutan dan peta file

Docker menjalankan file di `/docker-entrypoint-initdb.d` menurut urutan nama, hanya saat data directory kosong
(`../Dockerfile`). Urutan nama = urutan ketergantungan.

| File | Isi | Bergantung pada |
|---|---|---|
| `01a_accounts.sql` | `users`, `owners`, `payments` | - |
| `01b_queues.sql` | `queues`, `queue_settings`, `counters`, `queue_displays` | 01a |
| `01c_tickets.sql` | `tickets` (+ dua index unik aturan bisnis), `ticket_notes` | 01a, 01b |
| `01d_history.sql` | `queue_history` (arsip, tanpa FK ke tabel live) | 01a |
| `01e_notifications.sql` | `push_subscriptions`, `notification_outbox` (`ticket_id` boleh NULL, kolom `queue_id`, index unik parsial `uq_outbox_kind_queue`, `kind` termasuk `queue-closed`) | 01a, 01b, 01c |
| `01f_profile_security.sql` | `user_avatars`, `user_action_tokens` | 01a |
| `01g_triggers.sql` | fungsi dan trigger `tickets_notify`, `tickets_email_outbox`, `queues_email_outbox` (email ke owner saat antrian berubah dari `active` ke `closed`, mengikuti `users.email_notifications`) | 01a, 01b, 01c, 01e |
| `01i_payment_orders.sql` | `payment_orders` (pesanan checkout BorderPay; `status` pending, paid, expired, failed) | 01a |
| `01h_role_grants.sql` | blok `GRANT` bersyarat untuk role `mail_rw` | 01a, 01e |
| `02_Indexes.sql` | semua index non-unik, dikelompokkan per tabel | seluruh `01*` |
| `03_notify_role.sh` | role `notify_ro` (notify-service) | seluruh `01*` |
| `04_mail_role.sh` | role `mail_rw` (mail-service) | seluruh `01*` |
| `06_auth_role.sh` | role `auth_rw` (auth-service): `users` dan `user_action_tokens` saja | seluruh `01*` |
| `07_metrics_role.sh` | role `metrics_ro` (postgres_exporter): hanya `pg_monitor`, tanpa akses ke tabel data. Dilewati bila `METRICS_DB_PASSWORD` kosong; `make obs-up` menjalankannya idempoten di database yang sudah ada | - |

Menjalankan manual di database kosong (tanpa Docker):

```bash
createdb qms_db
for f in db/migrations/01*.sql db/migrations/02_Indexes.sql; do psql -d qms_db -v ON_ERROR_STOP=1 -f "$f"; done
```

## Aturan

- Skema file `01*` **tidak idempoten** (`CREATE` tanpa `IF NOT EXISTS`). Menjalankannya dua kali gagal. `02_Indexes.sql`
  idempoten (`IF NOT EXISTS`).
- **Jangan ganti nama** tabel, kolom, constraint, atau index tanpa migrasi. Nama yang sama dipakai database yang sudah
  berjalan dan `apply_schema.sql`. Beberapa nama tidak seragam (mis. `user_action_tokens_user_idx` tanpa awalan `idx_`)
  karena alasan itu.
- Index non-unik hanya di `02_Indexes.sql`. Constraint `PRIMARY KEY` dan `UNIQUE`, serta dua index unik parsial yang
  menegakkan aturan bisnis (`tickets_queue_id_user_id_active_key`, `tickets_one_serving_per_counter`), ada di file tabelnya.
- Isi fungsi trigger tersimpan apa adanya di katalog Postgres, **termasuk komentar di dalam badan fungsi**. Mengubah
  satu karakter di badan fungsi berarti mengubah skema dan perlu diterapkan juga ke `apply_schema.sql`.
- Tes integrasi backend memuat semua `01*.sql` lalu `02_Indexes.sql` ke schema sementara (urut menurut nama). Jangan
  memakai nama berawalan schema (`public.xxx`) atau perintah tingkat database (`CREATE DATABASE`, `\connect`).
- Skrip role (`03..07`) memberi hak sendiri. Tabel berisi data sensitif (`user_avatars`, `user_action_tokens`,
  `queue_displays`) sengaja tidak diberi hak ke role mana pun selain pemilik, kecuali `user_action_tokens` untuk `auth_rw`.

## Menambah atau mengubah skema

1. Ubah tabel di file domainnya (atau tambah file `01x_nama.sql` baru dengan huruf yang menjaga urutan ketergantungan;
   perbarui tabel di atas dan daftar `COPY` bila perlu). Tambah index-nya di `02_Indexes.sql`.
2. Tambahkan perubahan yang sama dalam bentuk idempoten ke `../scripts/apply_schema.sql`.
3. Bila tabel atau kolom baru dibaca service lain, perbarui hak di skrip role yang bersangkutan.
4. Buktikan bahwa database baru dan database yang sudah di-upgrade punya katalog yang sama (bandingkan `pg_dump
   --schema-only` keduanya).

## Kolom yang ada tetapi tidak dipakai kode

`queue_settings`: `registration_limit`, `display_next_count`, `display_skipped_count`, `fallback_mode`,
`fallback_offset`, `fallback_max_attempts`, `last_rebalanced_at`. Fitur dihapus dari produk (31 Okt 2026) atau
digantikan tabel lain. Dipertahankan agar skema sama dengan database yang sudah berjalan; menghapusnya butuh migrasi
di `apply_schema.sql`.
