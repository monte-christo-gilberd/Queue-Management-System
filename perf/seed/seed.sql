-- =============================================================================
-- seed.sql: data uji beban massal (perf/). Dijalankan oleh `make perf-seed` di database uji (project qms-perf), BUKAN
-- database dev atau prod.
-- -----------------------------------------------------------------------------
-- Variabel psql (diisi `make perf-seed`, ada nilai bawaan):
--   owners         jumlah owner (bawaan 10)
--   customers      jumlah pelanggan (bawaan 1000)
--   queues         jumlah antrian, dibagi rata ke owner (bawaan 10)
--   password       password SEMUA akun seed (bawaan perf-password); dipakai uji login nyata
-- Hasil: user perf-owner-<n>@perf.invalid dan perf-customer-<n>@perf.invalid, semua email terverifikasi dan
-- email_notifications mati (uji tidak boleh menulis outbox email), tiap owner berlangganan 365 hari, tiap antrian punya
-- counter default dan pengaturan bawaan.
-- Registrasi lewat API memakai bcrypt dan terlalu lambat untuk ribuan akun, jadi akun diisi langsung lewat SQL. Tabel users
-- mewajibkan password_hash atau google_id (constraint users_check), jadi SEMUA akun diberi hash yang sama: bcrypt dihitung
-- SEKALI (cost 10, sama dengan auth-service) lalu dipakai ulang, jadi seed ribuan akun tetap cepat. Sesi uji memakai JWT
-- buatan perf/seed/mint; password hanya dipakai bagian kecil lalu lintas login nyata.
-- Idempoten: bila data perf-* sudah ada, skrip berhenti tanpa mengubah apa pun (hapus volume project qms-perf untuk reset).
-- =============================================================================
\set ON_ERROR_STOP on
\if :{?owners} \else \set owners 10 \endif
\if :{?customers} \else \set customers 1000 \endif
\if :{?queues} \else \set queues 10 \endif
\if :{?password} \else \set password perf-password \endif

-- pgcrypto menyediakan crypt() dan gen_salt('bf') untuk hash bcrypt yang kompatibel dengan bcrypt Go di auth-service
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- hash bcrypt dihitung sekali dan disimpan di variabel psql pw_hash (dipakai semua akun)
SELECT crypt(:'password', gen_salt('bf', 10)) AS pw_hash \gset

-- berhenti bila seed sudah pernah dijalankan (cek satu akun)
SELECT EXISTS (SELECT 1 FROM users WHERE email = 'perf-owner-1@perf.invalid') AS sudah_ada \gset
\if :sudah_ada
  \echo 'Seed perf sudah ada: tidak ada yang diubah (hapus volume project qms-perf untuk reset).'
  \quit
\endif

BEGIN;

-- owner: id terurut sesuai nomor
INSERT INTO users (email, username, password_hash, email_verified_at, email_notifications)
SELECT 'perf-owner-' || g || '@perf.invalid', 'perf-owner-' || g,
       :'pw_hash', now(), false
FROM generate_series(1, :owners) g;

-- pelanggan: id berikutnya, terurut sesuai nomor
INSERT INTO users (email, username, password_hash, email_verified_at, email_notifications)
SELECT 'perf-customer-' || g || '@perf.invalid', 'perf-customer-' || g,
       :'pw_hash', now(), false
FROM generate_series(1, :customers) g;

-- langganan aktif untuk setiap owner (tanpa ini join dan pembuatan antrian ditolak 402)
INSERT INTO owners (user_id, subscription_expires_at)
SELECT id, now() + interval '365 days' FROM users WHERE email LIKE 'perf-owner-%@perf.invalid';

-- antrian: dibagi rata ke owner secara berputar
INSERT INTO queues (owner_id, queue_name, queue_desc, ticket_prefix)
SELECT o.id, 'Perf ' || g, 'antrian uji beban', 'P'
FROM generate_series(1, :queues) g
JOIN (
    SELECT id, row_number() OVER (ORDER BY id) AS n FROM users WHERE email LIKE 'perf-owner-%@perf.invalid'
) o ON o.n = ((g - 1) % :owners) + 1;

-- pengaturan dan counter default tiap antrian (sama dengan yang dibuat backend saat antrian dibuat lewat API)
INSERT INTO queue_settings (queue_id) SELECT id FROM queues WHERE queue_name LIKE 'Perf %';
INSERT INTO counters (queue_id, name, is_default) SELECT id, 'Counter 1', true FROM queues WHERE queue_name LIKE 'Perf %';

COMMIT;

\echo 'Seed selesai.'
SELECT (SELECT count(*) FROM users WHERE email LIKE 'perf-owner-%@perf.invalid') AS owners,
       (SELECT count(*) FROM users WHERE email LIKE 'perf-customer-%@perf.invalid') AS customers,
       (SELECT count(*) FROM queues WHERE queue_name LIKE 'Perf %') AS queues;
