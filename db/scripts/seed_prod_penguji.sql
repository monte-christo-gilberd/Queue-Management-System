-- =============================================================================
-- SEED PRODUKSI UNTUK PENGUJI: 1 akun (owner + customer) dan data antrian uji.
-- -----------------------------------------------------------------------------
-- Dipakai oleh:
--   scripts/seed-prod-penguji.sh     membuat akun penguji lewat API (bila belum ada), lalu menjalankan berkas ini
--   scripts/restore-prod-penguji.sh  hanya menjalankan berkas ini: data uji kembali ke kondisi seed
-- Butuh dua variabel psql:  -v tester_email=... -v tester_username=...
-- Akun penguji HARUS sudah ada di users (berkas ini menolak jalan bila belum).
--
-- Yang DISENTUH, dan hanya ini:
--   1. akun penguji: username, notifikasi email, verifikasi, langganan, foto profil, langganan push, pembayaran
--   2. antrian, tiket, counter, layar publik, catatan, dan riwayat milik akun penguji
--   3. tiket akun penguji di antrian mana pun (akun ini hanya untuk uji)
--   4. akun pelanggan seed: users dengan google_id 'seed-pelanggan-N' (tanpa password, tidak bisa login)
-- Kata sandi penguji TIDAK diubah. Akun lain dan data produksi lain tidak disentuh.
--
-- Isi setelah dijalankan:
--   Antrian aktif "Antrian Uji Penguji" (awalan P):
--     nomor 1 sedang dilayani di Loket 1; nomor 2, 3, 5, 8 menunggu; nomor 4 dilewati; nomor 6 selesai di Loket 2;
--     nomor 7 dibatalkan. Dua counter. Skip: kembali ke belakang. Layar publik aktif. Catatan di nomor 2 dan 4.
--     Akun penguji BELUM punya nomor: bergabung sendiri lewat QR (boleh memindai QR antriannya sendiri).
--   Antrian tutup "Antrian Uji Sebelumnya" (awalan R): tiga baris riwayat (selesai, dibatalkan, dilewati).
--   Pelanggan seed 1 sampai 8 ada di antrian aktif; 9 dan 10 belum (untuk uji Sisip).
--   Langganan penguji 365 hari dari waktu jalan berkas ini.
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
-- 3. Pelanggan seed. Tanpa password (password_hash NULL) dan notifikasi email mati, sehingga trigger tidak
--    menulis email untuk mereka. Hanya bisa dipakai sebagai tiket, tidak bisa login.
-- -----------------------------------------------------------------------------
INSERT INTO users (email, username, password_hash, google_id, email_notifications, email_verified_at)
SELECT 'seed-pelanggan-' || n || '@qms.invalid',
       'pelanggan-' || n,
       NULL,
       'seed-pelanggan-' || n,
       FALSE,
       now()
FROM generate_series(1, 10) AS n;

CREATE TEMP TABLE _pel ON COMMIT DROP AS
SELECT substring(google_id FROM '[0-9]+$')::int AS n, id
FROM users
WHERE google_id LIKE 'seed-pelanggan-%';

-- -----------------------------------------------------------------------------
-- 4. Antrian: satu aktif, satu sudah ditutup. kunci = 'aktif' atau 'tutup'.
-- -----------------------------------------------------------------------------
CREATE TEMP TABLE _q (kunci text PRIMARY KEY, id bigint NOT NULL) ON COMMIT DROP;

WITH q AS (
    INSERT INTO queues (owner_id, queue_name, queue_desc, ticket_prefix, status, created_at, closed_at)
    SELECT owner_id,
           'Antrian Uji Penguji',
           'Antrian aktif untuk pengujian. Akun ini adalah owner sekaligus customer.',
           'P', 'active', now() - interval '2 hours', NULL
    FROM _ctx
    RETURNING id
)
INSERT INTO _q (kunci, id) SELECT 'aktif', id FROM q;

WITH q AS (
    INSERT INTO queues (owner_id, queue_name, queue_desc, ticket_prefix, status, created_at, closed_at)
    SELECT owner_id,
           'Antrian Uji Sebelumnya',
           'Contoh antrian yang sudah ditutup, untuk menguji Riwayat dan ekspor.',
           'R', 'closed', now() - interval '3 days', now() - interval '2 days'
    FROM _ctx
    RETURNING id
)
INSERT INTO _q (kunci, id) SELECT 'tutup', id FROM q;

-- Pengaturan: skip "kembali ke belakang" pada antrian aktif, default pada antrian tutup.
INSERT INTO queue_settings (queue_id, skip_fallback, skip_fallback_n, skip_max_returns)
SELECT id, CASE kunci WHEN 'aktif' THEN 'back' ELSE 'remove' END, NULL, 1
FROM _q;

-- Counter: Loket 1 (default, tidak bisa dihapus) di kedua antrian; Loket 2 hanya di antrian aktif.
INSERT INTO counters (queue_id, name, created_at, is_default)
SELECT id, 'Loket 1', now() - interval '2 hours', TRUE FROM _q;

INSERT INTO counters (queue_id, name, created_at, is_default)
SELECT id, 'Loket 2', now() - interval '2 hours', FALSE FROM _q WHERE kunci = 'aktif';

CREATE TEMP TABLE _ctr ON COMMIT DROP AS
SELECT c.id, c.name, c.queue_id
FROM counters c
JOIN _q q ON q.id = c.queue_id;

-- Layar publik antrian aktif. Token 32 karakter hex (lolos validDisplayToken: panjang 32, huruf dan angka).
INSERT INTO queue_displays (queue_id, token, upcoming_count, skipped_count, created_at)
SELECT id, md5(gen_random_uuid()::text || gen_random_uuid()::text), 5, 3, now()
FROM _q WHERE kunci = 'aktif';

-- -----------------------------------------------------------------------------
-- 5. Tiket di antrian aktif. Kolom waktu dihitung relatif terhadap sekarang (menit lalu).
--    Ada tepat satu tiket currently_served per counter, sesuai indeks tickets_one_serving_per_counter.
-- -----------------------------------------------------------------------------
INSERT INTO tickets (queue_id, ticket_number, sort_key, user_id, status, skip_count,
                     called_counter_id, called_at, completed_at, cancelled_at, created_at)
SELECT q.id,
       v.nomor,
       v.nomor::bigint * 65536,
       p.id,
       v.status,
       v.skips,
       c.id,
       CASE WHEN v.dipanggil IS NOT NULL THEN now() - make_interval(mins => v.dipanggil) END,
       CASE WHEN v.status = 'completed' THEN now() - make_interval(mins => v.selesai) END,
       CASE WHEN v.status = 'cancelled' THEN now() - make_interval(mins => v.selesai) END,
       now() - make_interval(mins => v.dibuat)
FROM (VALUES
    (1, 1, 'currently_served', 0, 'Loket 1', 3,    NULL, 20),
    (2, 2, 'waiting',          0, NULL,      NULL, NULL, 18),
    (3, 3, 'waiting',          0, NULL,      NULL, NULL, 16),
    (4, 4, 'skipped',          1, NULL,      NULL, NULL, 14),
    (5, 5, 'waiting',          0, NULL,      NULL, NULL, 12),
    (6, 6, 'completed',        0, 'Loket 2', 9,    6,    10),
    (7, 7, 'cancelled',        0, NULL,      NULL, 4,    8),
    (8, 8, 'waiting',          0, NULL,      NULL, NULL, 2)
) AS v(nomor, pel, status, skips, ctr, dipanggil, selesai, dibuat)
JOIN _q q   ON q.kunci = 'aktif'
JOIN _pel p ON p.n = v.pel
LEFT JOIN _ctr c ON c.queue_id = q.id AND c.name = v.ctr;

-- Catatan owner (hanya terlihat owner) pada nomor 2 dan 4.
INSERT INTO ticket_notes (ticket_id, note, created_by, updated_by)
SELECT t.id, v.catatan, o.owner_id, o.owner_id
FROM (VALUES
    (2, 'Minta didahulukan, harus pulang lebih awal.'),
    (4, 'Tidak ada di tempat saat dipanggil.')
) AS v(nomor, catatan)
JOIN tickets t ON t.ticket_number = v.nomor
              AND t.queue_id = (SELECT id FROM _q WHERE kunci = 'aktif')
CROSS JOIN _ctx o;

-- -----------------------------------------------------------------------------
-- 6. Riwayat untuk antrian yang sudah ditutup (tabel queue_history tidak punya FK ke antrian).
-- -----------------------------------------------------------------------------
INSERT INTO queue_history (owner_id, original_queue_id, queue_name, queue_opened_at, queue_closed_at,
                           ticket_number, customer_name, customer_email, outcome,
                           joined_at, completed_at, cancelled_at, skip_count, note)
SELECT o.owner_id,
       q.id,
       'Antrian Uji Sebelumnya',
       now() - interval '3 days',
       now() - interval '2 days',
       v.nomor,
       'Pelanggan ' || p.n,
       'seed-pelanggan-' || p.n || '@qms.invalid',
       v.outcome,
       now() - interval '3 days' + make_interval(mins => v.nomor * 5),
       CASE WHEN v.outcome = 'completed'
            THEN now() - interval '3 days' + make_interval(mins => v.nomor * 5 + 10) END,
       CASE WHEN v.outcome = 'cancelled'
            THEN now() - interval '3 days' + make_interval(mins => v.nomor * 5 + 3) END,
       v.skips,
       v.catatan
FROM (VALUES
    (1, 1, 'completed', 0, NULL::text),
    (2, 2, 'cancelled', 0, NULL::text),
    (3, 3, 'skipped',   1, 'Tidak hadir saat dipanggil')
) AS v(nomor, pel, outcome, skips, catatan)
JOIN _q q   ON q.kunci = 'tutup'
JOIN _pel p ON p.n = v.pel
CROSS JOIN _ctx o;

-- -----------------------------------------------------------------------------
-- Ringkasan sebelum commit.
-- -----------------------------------------------------------------------------
SELECT q.id, q.queue_name, q.status, q.ticket_prefix,
       (SELECT COUNT(*) FROM tickets t WHERE t.queue_id = q.id) AS tiket
FROM queues q
WHERE q.owner_id = (SELECT owner_id FROM _ctx)
ORDER BY q.id;

SELECT d.token AS token_layar_publik
FROM queue_displays d
JOIN _q q ON q.id = d.queue_id;

COMMIT;
