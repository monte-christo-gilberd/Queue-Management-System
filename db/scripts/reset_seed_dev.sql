-- =============================================================================
-- RESET + SEED DATA DEVELOPMENT (DEV/TEST SAJA)
--
-- 1) Menghapus SEMUA data: semua akun (users, owners) dan semua data antrian.
-- 2) Mengisi seed: 2 akun owner dan 10 akun user. Semua password = "password".
--
-- Akun seed:
--   owner1@test.com  (username owner1)  owner, langganan aktif 30 hari
--   owner2@test.com  (username owner2)  owner, langganan aktif 30 hari
--   owner3@test.com  (username owner3)  owner, langganan kedaluwarsa, 1 antrian aktif (TC-04-03)
--   user1@test.com … user10@test.com    (username user1 … user10)  customer
--
-- PERINGATAN: ini menghapus SEMUA akun dan data. Jangan dijalankan di database produksi.
-- Jalankan dari folder queue-management-system:
--   set -a && . backend/.env && set +a
--   psql "$DATABASE_URL" -f db/scripts/reset_seed_dev.sql
-- =============================================================================

-- Hash bcrypt (cost 10) untuk password "password". Sama dengan yang diverifikasi backend.
\set pw '$2a$10$aa7OdpdaYDKx.qpwtPwIS.euoamhxVmmluFIGPVTlZ8dBCI4htSIi'

BEGIN;

-- Pengaman: menolak jalan di database yang ditandai produksi/demo.
-- Tandai DB demo sekali dengan:  CREATE TABLE _no_seed_guard (x int);
-- (make prod-guard melakukannya). Di dev tabel ini tidak ada.
DO $$
BEGIN
    IF to_regclass('public._no_seed_guard') IS NOT NULL THEN
        RAISE EXCEPTION 'Database ini ditandai produksi (_no_seed_guard). Seed/reset ditolak.';
    END IF;
END $$;

-- Kosongkan semua tabel yang punya data. Semua tabel yang mereferensi users/owners ikut dalam daftar,
-- sehingga tidak perlu CASCADE. Tanpa CASCADE, jika ada tabel lain yang bergantung, TRUNCATE gagal
-- dan tidak ada yang berubah. RESTART IDENTITY mengembalikan id mulai dari 1.
TRUNCATE TABLE
    notification_outbox,
    user_action_tokens,
    queue_displays,
    user_avatars,
    push_subscriptions,
    ticket_notes,
    counters,
    tickets,
    queue_history,
    queue_settings,
    queues,
    payments,
    owners,
    users
RESTART IDENTITY;

-- Akun user. id: owner1=1, owner2=2, owner3=3, user1..user10=4..13.
INSERT INTO users (email, username, password_hash) VALUES
    ('owner1@test.com', 'owner1', :'pw'),
    ('owner2@test.com', 'owner2', :'pw'),
    ('owner3@test.com', 'owner3', :'pw'),
    ('user1@test.com',  'user1',  :'pw'),
    ('user2@test.com',  'user2',  :'pw'),
    ('user3@test.com',  'user3',  :'pw'),
    ('user4@test.com',  'user4',  :'pw'),
    ('user5@test.com',  'user5',  :'pw'),
    ('user6@test.com',  'user6',  :'pw'),
    ('user7@test.com',  'user7',  :'pw'),
    ('user8@test.com',  'user8',  :'pw'),
    ('user9@test.com',  'user9',  :'pw'),
    ('user10@test.com', 'user10', :'pw');

-- Dua owner, dengan langganan aktif.
INSERT INTO owners (user_id, subscription_expires_at)
SELECT id, now() + interval '30 days'
FROM users
WHERE username IN ('owner1', 'owner2');

-- owner3: langganan sudah kedaluwarsa, tetapi antrian lamanya masih aktif.
INSERT INTO owners (user_id, subscription_expires_at)
SELECT id, now() - interval '1 day' FROM users WHERE username = 'owner3';

INSERT INTO queues (owner_id, queue_name, queue_desc)
SELECT id, 'Antrian Lama', 'Dibuat sebelum langganan berakhir' FROM users WHERE username = 'owner3';

INSERT INTO counters (queue_id, name, created_at, is_default)
SELECT id, 'Default', now(), TRUE FROM queues WHERE queue_name = 'Antrian Lama';

INSERT INTO queue_settings (queue_id)
SELECT id FROM queues WHERE queue_name = 'Antrian Lama';

-- Ringkasan sebelum commit.
SELECT
    (SELECT COUNT(*) FROM users)   AS users,
    (SELECT COUNT(*) FROM owners)  AS owners,
    (SELECT COUNT(*) FROM queues)  AS queues,
    (SELECT COUNT(*) FROM tickets) AS tickets;

COMMIT;
