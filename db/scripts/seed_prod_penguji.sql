-- =============================================================================
-- SEED PRODUKSI UNTUK PENGUJI: 1 akun (owner + customer) dan data antrian uji per fitur.
-- -----------------------------------------------------------------------------
-- Dipakai oleh:
--   scripts/seed-prod-penguji.sh     membuat akun penguji lewat API (bila belum ada), lalu menjalankan berkas ini
-- Butuh dua variabel psql:  -v tester_email=... -v tester_username=...
-- Akun penguji HARUS sudah ada di users (berkas ini menolak jalan bila belum).
--
-- Yang DISENTUH, dan hanya ini:
--   1. akun penguji: username, notifikasi email, verifikasi, langganan, foto profil, langganan push, pembayaran
--   2. antrian, tiket, counter, layar publik, catatan, dan riwayat milik akun penguji
--   3. tiket akun penguji di antrian mana pun (akun ini hanya untuk uji)
--   4. akun pelanggan seed: users dengan google_id 'seed-pelanggan-N' (kata sandi acak diberikan skrip setelah berkas ini)
-- Kata sandi penguji dan pelanggan diatur oleh scripts/seed-prod-penguji.sh (bukan oleh berkas ini). Akun lain dan data produksi lain tidak disentuh.
--
-- Isi setelah dijalankan: satu antrian per fitur. Setiap antrian aktif berisi 5 sampai 12 orang menunggu.
--   N  Demo NEXT             10 menunggu. Uji NEXT berulang.
--   U  Demo Panggil Ulang    1 dilayani (Loket 1), 6 menunggu. Uji PANGGIL ULANG.
--   S  Demo Skip             1 dilayani (Loket 1), 7 menunggu. Skip: kembali ke belakang.
--   H  Demo Hapus Nomor      8 menunggu. Uji hapus nomor oleh owner (customer dapat email dan layar berubah).
--   B  Demo Batal Nomor      6 menunggu, termasuk AKUN PENGUJI di nomor 3. Uji batal dari customer.
--   I  Demo Sisip            6 menunggu. Sisip pelanggan-7 sampai pelanggan-12 (belum antre).
--   C  Demo Catatan          6 menunggu. Catatan sudah ada di nomor 1, 3, dan 5.
--   K  Demo Counter          1 dilayani di Loket 2, 6 menunggu. Loket 1 (default) dan Loket 2 bisa diuji CRUD.
--   T  Demo Tutup Antrian    1 dilayani (Loket 1), 5 menunggu. Uji tutup antrian.
--   R  Riwayat Panjang       antrian sudah ditutup, 25 baris riwayat (selesai, batal, dilewati, dihapus).
-- Layar publik aktif di antrian N. Pelanggan seed: pelanggan-1 sampai pelanggan-12.
-- Langganan penguji 365 hari dari waktu jalan berkas ini.
-- =============================================================================

BEGIN;

-- Konteks: id akun penguji. Tabel sementara, hilang saat transaksi selesai.
CREATE TEMP TABLE _ctx ON COMMIT DROP AS
SELECT id AS owner_id FROM users WHERE email = :'tester_email';

DO $$
BEGIN
    IF (SELECT COUNT(*) FROM _ctx) <> 1 THEN
        RAISE EXCEPTION 'Akun penguji tidak ditemukan. Jalankan scripts/seed-prod-penguji.sh dulu.';
    END IF;
END $$;

-- -----------------------------------------------------------------------------
-- 1. Hapus data uji lama. Urutan eksplisit (tiket, lalu counter, lalu antrian), tanpa CASCADE, sehingga
--    aturan "currently_served wajib punya counter" tidak dilanggar saat counter dihapus.
-- -----------------------------------------------------------------------------
DELETE FROM notification_outbox
 WHERE user_id = (SELECT owner_id FROM _ctx)
    OR user_id IN (SELECT id FROM users WHERE google_id LIKE 'seed-pelanggan-%');

DELETE FROM tickets
 WHERE user_id = (SELECT owner_id FROM _ctx)
    OR user_id IN (SELECT id FROM users WHERE google_id LIKE 'seed-pelanggan-%')
    OR queue_id IN (SELECT id FROM queues WHERE owner_id = (SELECT owner_id FROM _ctx));

DELETE FROM queue_displays
 WHERE queue_id IN (SELECT id FROM queues WHERE owner_id = (SELECT owner_id FROM _ctx));

DELETE FROM queue_settings
 WHERE queue_id IN (SELECT id FROM queues WHERE owner_id = (SELECT owner_id FROM _ctx));

DELETE FROM counters
 WHERE queue_id IN (SELECT id FROM queues WHERE owner_id = (SELECT owner_id FROM _ctx));

DELETE FROM queues WHERE owner_id = (SELECT owner_id FROM _ctx);
DELETE FROM queue_history WHERE owner_id = (SELECT owner_id FROM _ctx);
DELETE FROM payments WHERE owner_id = (SELECT owner_id FROM _ctx);
DELETE FROM push_subscriptions WHERE user_id = (SELECT owner_id FROM _ctx);
DELETE FROM user_avatars WHERE user_id = (SELECT owner_id FROM _ctx);
DELETE FROM users WHERE google_id LIKE 'seed-pelanggan-%';

-- -----------------------------------------------------------------------------
-- 2. Akun penguji: kembalikan profil ke nilai awal, verifikasi email, langganan 365 hari.
-- -----------------------------------------------------------------------------
UPDATE users
   SET username            = :'tester_username',
       email_notifications = TRUE,
       email_verified_at   = COALESCE(email_verified_at, now())
 WHERE id = (SELECT owner_id FROM _ctx);

INSERT INTO owners (user_id, subscription_expires_at)
SELECT owner_id, now() + interval '365 days' FROM _ctx
ON CONFLICT (user_id) DO UPDATE SET subscription_expires_at = EXCLUDED.subscription_expires_at;

-- -----------------------------------------------------------------------------
-- 3. Pelanggan seed (12 orang). Dibuat tanpa password (password_hash NULL); scripts/seed-prod-penguji.sh memberi
--    kata sandi acak setelah berkas ini. Notifikasi email mati, sehingga trigger tidak menulis email untuk mereka.
-- -----------------------------------------------------------------------------
INSERT INTO users (email, username, password_hash, google_id, email_notifications, email_verified_at)
SELECT 'seed-pelanggan-' || n || '@qms.invalid',
       'pelanggan-' || n,
       NULL,
       'seed-pelanggan-' || n,
       FALSE,
       now()
FROM generate_series(1, 12) AS n;

-- n = nomor pelanggan (1..12). id = users.id. Baris n = 0 adalah akun penguji (dipakai sebagai customer di antrian B).
CREATE TEMP TABLE _pel ON COMMIT DROP AS
SELECT substring(google_id FROM '[0-9]+$')::int AS n, id
FROM users
WHERE google_id LIKE 'seed-pelanggan-%';

INSERT INTO _pel (n, id) SELECT 0, owner_id FROM _ctx;

-- -----------------------------------------------------------------------------
-- 4. Antrian: satu per fitur. kunci = awalan tiket (unik per antrian).
-- -----------------------------------------------------------------------------
CREATE TEMP TABLE _q (kunci text PRIMARY KEY, id bigint NOT NULL) ON COMMIT DROP;

INSERT INTO queues (owner_id, queue_name, queue_desc, ticket_prefix, status, created_at, closed_at)
SELECT o.owner_id, v.nama, v.deskripsi, v.kunci, v.status,
       now() - make_interval(mins => v.dibuat_menit),
       CASE WHEN v.status = 'closed' THEN now() - interval '1 day' END
FROM _ctx o
CROSS JOIN (VALUES
    ('N', 'Demo NEXT',          'Uji tombol NEXT. 10 orang menunggu.',                                   'active', 180),
    ('U', 'Demo Panggil Ulang', 'Uji PANGGIL ULANG pada nomor yang sedang dilayani.',                    'active', 180),
    ('S', 'Demo Skip',          'Uji SKIP. Pengaturan skip: kembali ke belakang antrian.',               'active', 180),
    ('H', 'Demo Hapus Nomor',   'Uji hapus nomor oleh owner. Customer dapat email dan layarnya berubah.', 'active', 180),
    ('B', 'Demo Batal Nomor',   'Uji batal dari customer. Akun penguji sudah punya nomor di antrian ini.', 'active', 180),
    ('I', 'Demo Sisip',         'Uji sisip customer. Gunakan username pelanggan-7 sampai pelanggan-12.', 'active', 180),
    ('C', 'Demo Catatan',       'Uji catatan per nomor. Catatan hanya terlihat owner.',                  'active', 180),
    ('K', 'Demo Counter',       'Uji tambah, ganti nama, dan hapus counter. Loket 2 sedang melayani nomor 1.', 'active', 180),
    ('T', 'Demo Tutup Antrian', 'Uji tutup antrian. Setelah ditutup, antrian pindah ke riwayat.',       'active', 180),
    ('R', 'Riwayat Panjang',    'Antrian sudah ditutup dengan 25 nomor, untuk uji paginasi Riwayat dan ekspor.', 'closed', 2880)
) AS v(kunci, nama, deskripsi, status, dibuat_menit);

INSERT INTO _q (kunci, id)
SELECT ticket_prefix, id FROM queues WHERE owner_id = (SELECT owner_id FROM _ctx);

-- Pengaturan: skip "kembali ke belakang" hanya di antrian S. Lainnya default (hapus).
INSERT INTO queue_settings (queue_id, skip_fallback, skip_max_returns)
SELECT id, CASE kunci WHEN 'S' THEN 'back' ELSE 'remove' END, 1
FROM _q;

-- Counter: Loket 1 (default, tidak bisa dihapus) di semua antrian; Loket 2 hanya di antrian K.
INSERT INTO counters (queue_id, name, created_at, is_default)
SELECT id, 'Loket 1', now() - interval '3 hours', TRUE FROM _q;

INSERT INTO counters (queue_id, name, created_at, is_default)
SELECT id, 'Loket 2', now() - interval '3 hours', FALSE FROM _q WHERE kunci = 'K';

CREATE TEMP TABLE _ctr ON COMMIT DROP AS
SELECT c.id, c.name, c.queue_id
FROM counters c
JOIN _q q ON q.id = c.queue_id;

-- Layar publik hanya di antrian N. Token 32 karakter hex (lolos validDisplayToken: panjang 32, huruf dan angka).
INSERT INTO queue_displays (queue_id, token, upcoming_count, skipped_count, created_at)
SELECT id, md5(gen_random_uuid()::text || gen_random_uuid()::text), 5, 3, now()
FROM _q WHERE kunci = 'N';

-- -----------------------------------------------------------------------------
-- 5. Tiket. Waktu dihitung relatif terhadap sekarang (menit lalu). Nomor dilayani dulu, lalu yang menunggu.
--    Nomor menunggu = pelanggan dengan nomor yang sama (pelanggan-N), kecuali antrian B nomor 3 = akun penguji.
--    Satu tiket currently_served per counter, sesuai indeks tickets_one_serving_per_counter.
-- -----------------------------------------------------------------------------
INSERT INTO tickets (queue_id, ticket_number, sort_key, user_id, status, skip_count,
                     called_counter_id, called_at, completed_at, cancelled_at, created_at)
SELECT q.id, v.nomor, v.nomor::bigint * 65536, p.id, 'currently_served', 0,
       c.id, now() - interval '2 minutes', NULL, NULL,
       now() - make_interval(mins => 60 - 3 * v.nomor)
FROM (VALUES
    ('U', 1, 1, 'Loket 1'),
    ('S', 1, 1, 'Loket 1'),
    ('K', 1, 1, 'Loket 2'),
    ('T', 1, 1, 'Loket 1')
) AS v(kunci, nomor, pel, ctr)
JOIN _q q   ON q.kunci = v.kunci
JOIN _pel p ON p.n = v.pel
JOIN _ctr c ON c.queue_id = q.id AND c.name = v.ctr;

INSERT INTO tickets (queue_id, ticket_number, sort_key, user_id, status, skip_count,
                     called_counter_id, called_at, completed_at, cancelled_at, created_at)
SELECT q.id, r.nomor, r.nomor::bigint * 65536, p.id, 'waiting', 0,
       NULL, NULL, NULL, NULL,
       now() - make_interval(mins => 60 - 3 * r.nomor)
FROM (VALUES
    ('N', 1, 10),
    ('U', 2, 7),
    ('S', 2, 8),
    ('H', 1, 8),
    ('B', 1, 6),
    ('I', 1, 6),
    ('C', 1, 6),
    ('K', 2, 7),
    ('T', 2, 6)
) AS a(kunci, awal, akhir)
CROSS JOIN LATERAL generate_series(a.awal, a.akhir) AS r(nomor)
JOIN _q q   ON q.kunci = a.kunci
JOIN _pel p ON p.n = CASE WHEN a.kunci = 'B' AND r.nomor = 3 THEN 0 ELSE r.nomor END;

-- Catatan owner (hanya terlihat owner) pada antrian C nomor 1, 3, dan 5.
INSERT INTO ticket_notes (ticket_id, note, created_by, updated_by)
SELECT t.id, v.catatan, o.owner_id, o.owner_id
FROM (VALUES
    (1, 'Minta dipanggil di loket dekat pintu.'),
    (3, 'Membawa anak kecil, mohon didahulukan.'),
    (5, 'Sudah menghubungi owner lewat telepon.')
) AS v(nomor, catatan)
JOIN tickets t ON t.ticket_number = v.nomor
              AND t.queue_id = (SELECT id FROM _q WHERE kunci = 'C')
CROSS JOIN _ctx o;

-- -----------------------------------------------------------------------------
-- 6. Riwayat panjang untuk antrian R (tabel queue_history tidak punya FK ke antrian). 25 baris.
-- -----------------------------------------------------------------------------
INSERT INTO queue_history (owner_id, original_queue_id, queue_name, queue_opened_at, queue_closed_at,
                           ticket_number, customer_name, customer_email, outcome,
                           joined_at, completed_at, cancelled_at, skip_count, note)
SELECT o.owner_id,
       q.id,
       'Riwayat Panjang',
       now() - interval '2 days',
       now() - interval '1 day',
       h.n,
       'Pelanggan ' || h.pel,
       'seed-pelanggan-' || h.pel || '@qms.invalid',
       h.outcome,
       now() - interval '2 days' + make_interval(mins => h.n * 40),
       CASE WHEN h.outcome = 'completed'
            THEN now() - interval '2 days' + make_interval(mins => h.n * 40 + 10) END,
       CASE WHEN h.outcome = 'cancelled'
            THEN now() - interval '2 days' + make_interval(mins => h.n * 40 + 3) END,
       CASE WHEN h.outcome = 'skipped' THEN 1 ELSE 0 END,
       CASE WHEN h.n % 4 = 0 THEN 'Catatan contoh untuk nomor ' || h.n END
FROM (
    SELECT n,
           ((n - 1) % 12) + 1 AS pel,
           CASE WHEN n % 5 = 0 THEN 'cancelled'
                WHEN n % 7 = 0 THEN 'skipped'
                WHEN n % 9 = 0 THEN 'deleted'
                ELSE 'completed' END AS outcome
    FROM generate_series(1, 25) AS n
) h
CROSS JOIN _ctx o
JOIN _q q ON q.kunci = 'R';

-- 6b. Nomor R juga ada di tabel tickets (antrian R sudah tertutup). Halaman detail antrian tertutup membaca tickets,
--     bukan queue_history, jadi tanpa blok ini daftar nomor di Riwayat Panjang kosong. Status sama dengan arsip di atas.
INSERT INTO tickets (queue_id, ticket_number, sort_key, user_id, status, skip_count,
                     called_counter_id, called_at, completed_at, cancelled_at, created_at)
SELECT q.id, h.n, h.n::bigint * 65536, p.id, h.st,
       CASE WHEN h.st = 'skipped' THEN 1 ELSE 0 END,
       NULL, NULL,
       CASE WHEN h.st = 'completed' THEN now() - interval '2 days' + make_interval(mins => h.n * 40 + 10) END,
       CASE WHEN h.st = 'cancelled' THEN now() - interval '2 days' + make_interval(mins => h.n * 40 + 3) END,
       now() - interval '2 days' + make_interval(mins => h.n * 40)
FROM (
    SELECT n,
           ((n - 1) % 12) + 1 AS pel,
           CASE WHEN n % 5 = 0 THEN 'cancelled'
                WHEN n % 7 = 0 THEN 'skipped'
                WHEN n % 9 = 0 THEN 'deleted'
                ELSE 'completed' END AS st
    FROM generate_series(1, 25) AS n
) h
JOIN _pel p ON p.n = h.pel
JOIN _q q ON q.kunci = 'R';

-- Insert tiket di atas memicu trigger email "joined" untuk akun penguji. Itu email palsu dari proses seed, jadi dibuang.
DELETE FROM notification_outbox WHERE user_id = (SELECT owner_id FROM _ctx);

-- -----------------------------------------------------------------------------
-- Ringkasan sebelum commit: jumlah menunggu dan dilayani per antrian.
-- -----------------------------------------------------------------------------
SELECT q.ticket_prefix AS awalan,
       q.queue_name,
       q.status,
       COUNT(t.id) FILTER (WHERE t.status = 'waiting')          AS menunggu,
       COUNT(t.id) FILTER (WHERE t.status = 'currently_served') AS dilayani
FROM queues q
LEFT JOIN tickets t ON t.queue_id = q.id
WHERE q.owner_id = (SELECT owner_id FROM _ctx)
GROUP BY q.id, q.ticket_prefix, q.queue_name, q.status
ORDER BY q.id;

SELECT d.token AS token_layar_publik
FROM queue_displays d
JOIN _q q ON q.id = d.queue_id;

COMMIT;
