-- Penyesuaian skema idempoten untuk DB yang SUDAH berjalan (tidak ada migration runner).
-- Aman dijalankan berulang. Skema saja yang diubah, kecuali beberapa UPDATE data yang idempoten (menutup antrian lama,
-- menandai email terverifikasi, mengosongkan kode QR lama). Untuk DB baru, db/migrations/01*.sql sudah lengkap.
--   dev:  docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
--   prod: docker compose -f docker-compose.prod.yml --env-file .env.prod exec -T db \
--           psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
-- Setelah itu restart backend dan notify-service. Backup dulu di prod.
BEGIN;

ALTER TABLE tickets ADD COLUMN IF NOT EXISTS recalled_at TIMESTAMPTZ;
ALTER TABLE queues ADD COLUMN IF NOT EXISTS ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A';
ALTER TABLE queue_settings ADD COLUMN IF NOT EXISTS skip_fallback VARCHAR NOT NULL DEFAULT 'back';
ALTER TABLE queue_settings ALTER COLUMN skip_fallback SET DEFAULT 'back';
ALTER TABLE queue_settings ADD COLUMN IF NOT EXISTS skip_fallback_n INT;
ALTER TABLE queue_settings ADD COLUMN IF NOT EXISTS skip_max_returns INT NOT NULL DEFAULT 1;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'queues_ticket_prefix_check') THEN
        ALTER TABLE queues ADD CONSTRAINT queues_ticket_prefix_check CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'queue_settings_skip_fallback_check') THEN
        ALTER TABLE queue_settings ADD CONSTRAINT queue_settings_skip_fallback_check
            CHECK (skip_fallback IN ('remove', 'back', 'after_n'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'queue_settings_skip_fallback_n_check') THEN
        ALTER TABLE queue_settings ADD CONSTRAINT queue_settings_skip_fallback_n_check
            CHECK (skip_fallback_n IS NULL OR skip_fallback_n >= 1);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'queue_settings_skip_max_returns_check') THEN
        ALTER TABLE queue_settings ADD CONSTRAINT queue_settings_skip_max_returns_check CHECK (skip_max_returns >= 1);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user ON push_subscriptions(user_id);

-- Token FCM perangkat Android (01j_push_devices.sql).
CREATE TABLE IF NOT EXISTS push_devices (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform    TEXT NOT NULL CHECK (platform IN ('android')),
    token       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_push_devices_user ON push_devices (user_id);

-- Status 'currently_served' (nomor yang sedang dilayani) menggantikan 'completed' sebagai sumber sinyal 'called'.
-- Kondisi 'completed' sengaja dibiarkan selama masa transisi: urutan deploy tidak penting.
-- Setelah service baru berjalan dan backfill terverifikasi, persempit menjadi = 'currently_served'.
CREATE OR REPLACE FUNCTION tickets_notify() RETURNS trigger AS $fn$
BEGIN
    -- requeued harus dicek paling awal: skip dengan fallback juga menaikkan skip_count, tetapi hasilnya nomor
    -- kembali menunggu, bukan skipped. Asal status bisa completed (Skip menurunkan nomor lebih dulu),
    -- currently_served (counter dihapus), atau skipped. reason membedakan dua sumber; counter dibawa di payload
    -- karena counter yang dihapus sudah tidak ada saat notify-service membaca.
    IF NEW.status = 'waiting' AND OLD.status IN ('currently_served', 'completed', 'skipped') THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'requeued', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id,
            'reason', CASE WHEN NEW.skip_count > OLD.skip_count THEN 'skip' ELSE 'counter_closed' END,
            'counter', (SELECT c.name FROM counters c WHERE c.id = OLD.called_counter_id))::text);
    ELSIF NEW.skip_count > OLD.skip_count THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'skipped', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id)::text);
    ELSIF NEW.status IN ('completed', 'currently_served') AND NEW.called_at IS NOT NULL
       AND NEW.called_at IS DISTINCT FROM OLD.called_at THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'called', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id)::text);
    ELSIF NEW.recalled_at IS NOT NULL AND NEW.recalled_at IS DISTINCT FROM OLD.recalled_at THEN
        PERFORM pg_notify('queue_events', json_build_object(
            'kind', 'recalled', 'queue_id', NEW.queue_id, 'ticket_id', NEW.id)::text);
    END IF;
    RETURN NULL;
END;
$fn$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tickets_notify_trg ON tickets;
CREATE TRIGGER tickets_notify_trg
    AFTER UPDATE OF status, called_at, recalled_at, skip_count ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_notify();

-- Status currently_served dan deleted (deleted dipakai saat owner menghapus tiket dari antrian; trigger email mencatatnya sebagai 'removed').
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_status_check;
ALTER TABLE tickets ADD CONSTRAINT tickets_status_check
    CHECK (status IN ('waiting', 'currently_served', 'completed', 'cancelled', 'skipped', 'deleted'));

-- Rejoin: UNIQUE seumur hidup diganti index parsial untuk tiket aktif saja.
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_queue_id_user_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS tickets_queue_id_user_id_active_key
    ON tickets (queue_id, user_id)
    WHERE status IN ('waiting', 'currently_served');

-- Role notify_ro dibuat oleh db/migrations/03_notify_role.sh (butuh password); di sini hanya hak aksesnya.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'notify_ro') THEN
        GRANT SELECT ON tickets, queues, counters, push_subscriptions, push_devices TO notify_ro;
        GRANT DELETE ON push_subscriptions, push_devices TO notify_ro;
    END IF;
END $$;

-- Multi-counter: satu counter melayani paling banyak satu nomor; nomor yang sedang dilayani wajib punya counter.
-- CHECK dipasang NOT VALID agar tidak memindai ulang tabel di sini. Sebelum VALIDATE CONSTRAINT, pastikan
-- tidak ada counter ganda dan tidak ada currently_served tanpa counter (lihat query cek di riwayat commit).
CREATE UNIQUE INDEX IF NOT EXISTS tickets_one_serving_per_counter
    ON tickets (called_counter_id)
    WHERE status = 'currently_served';

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'tickets_serving_has_counter') THEN
        ALTER TABLE tickets ADD CONSTRAINT tickets_serving_has_counter
            CHECK (status <> 'currently_served' OR called_counter_id IS NOT NULL) NOT VALID;
    END IF;
END $$;

-- Riwayat antrian: CloseQueue dulu tidak mengisi closed_at, sehingga antrian yang sudah ditutup tidak punya
-- tanggal tutup. Isi dengan PERKIRAAN dari aktivitas tiket terakhir (atau created_at bila tanpa tiket).
-- Antrian yang ditutup sesudah perbaikan kode memakai waktu tutup yang sebenarnya. Idempoten: hanya baris NULL.
UPDATE queues q
SET closed_at = COALESCE(
    (SELECT MAX(GREATEST(t.created_at, t.called_at, t.completed_at, t.cancelled_at))
       FROM tickets t WHERE t.queue_id = q.id),
    q.created_at)
WHERE q.status = 'closed' AND q.closed_at IS NULL;

-- Foto profil: JPEG 512x512 hasil olahan server, satu per user. Disimpan di database supaya ikut backup.
-- Batas 150 KB dijaga CHECK; notify_ro sengaja tidak diberi hak apa pun di tabel ini.
CREATE TABLE IF NOT EXISTS user_avatars (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL DEFAULT 'image/jpeg',
    data         BYTEA NOT NULL CHECK (octet_length(data) <= 153600),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Email ke pemilik tiket (item 8): preferensi dan outbox. Trigger menulis outbox DALAM transaksi yang sama
-- dengan perubahan tiket, jadi email tidak hilang walau mail-service mati; mail-service (role mail_rw) mengirimnya.
-- Terpisah dari tickets_notify: push tidak bergantung pada email sama sekali.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_notifications BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE IF NOT EXISTS notification_outbox (
    id              BIGSERIAL PRIMARY KEY,
    kind            TEXT NOT NULL CHECK (kind IN ('joined', 'cancelled', 'removed', 'completed', 'queue-closed')),
    ticket_id       BIGINT REFERENCES tickets(id) ON DELETE CASCADE,
    queue_id        BIGINT REFERENCES queues(id) ON DELETE CASCADE,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload         JSONB NOT NULL,   -- snapshot: queue_id, queue_name, prefix, number, name (tanpa catatan owner)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ,
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    failed_at       TIMESTAMPTZ,      -- gagal permanen (5xx atau batas percobaan): tidak dicoba lagi
    UNIQUE (kind, ticket_id)
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON notification_outbox (next_attempt_at)
    WHERE sent_at IS NULL AND failed_at IS NULL;

-- Email queue-closed ke owner saat antrian ditutup: tabel lama perlu kind baru, ticket_id boleh NULL,
-- dan kolom queue_id. CHECK bawaan kolom bernama notification_outbox_kind_check; dibuang lalu dipasang ulang.
ALTER TABLE notification_outbox DROP CONSTRAINT IF EXISTS notification_outbox_kind_check;
ALTER TABLE notification_outbox ADD CONSTRAINT notification_outbox_kind_check
    CHECK (kind IN ('joined', 'cancelled', 'removed', 'completed', 'queue-closed'));
ALTER TABLE notification_outbox ALTER COLUMN ticket_id DROP NOT NULL;
ALTER TABLE notification_outbox ADD COLUMN IF NOT EXISTS queue_id BIGINT REFERENCES queues(id) ON DELETE CASCADE;
-- Satu antrian paling banyak satu email queue-closed.
CREATE UNIQUE INDEX IF NOT EXISTS uq_outbox_kind_queue
    ON notification_outbox (kind, queue_id)
    WHERE queue_id IS NOT NULL;

CREATE OR REPLACE FUNCTION tickets_email_outbox() RETURNS trigger AS $fn$
DECLARE
    k TEXT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        k := 'joined';
    ELSIF NEW.status = 'cancelled' AND OLD.status IS DISTINCT FROM 'cancelled' THEN
        k := 'cancelled';
    ELSIF NEW.status = 'deleted' AND OLD.status IS DISTINCT FROM 'deleted' THEN
        k := 'removed';
    ELSIF NEW.status = 'completed' AND OLD.status = 'currently_served' THEN
        k := 'completed';
    ELSE
        RETURN NULL;
    END IF;

    -- Sakelar pengguna dibaca saat kejadian: mematikan sakelar tidak membatalkan email lama,
    -- tetapi kejadian sesudahnya tidak masuk outbox.
    INSERT INTO notification_outbox (kind, ticket_id, user_id, payload)
    SELECT k, NEW.id, u.id, jsonb_build_object(
        'queue_id', q.id, 'queue_name', q.queue_name, 'prefix', q.ticket_prefix,
        'number', NEW.ticket_number, 'name', u.username)
    FROM users u JOIN queues q ON q.id = NEW.queue_id
    WHERE u.id = NEW.user_id AND u.email_notifications
    ON CONFLICT (kind, ticket_id) DO NOTHING;

    IF FOUND THEN
        PERFORM pg_notify('mail_outbox', '');
    END IF;
    RETURN NULL;
END;
$fn$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tickets_email_outbox_trg ON tickets;
CREATE TRIGGER tickets_email_outbox_trg
    AFTER INSERT OR UPDATE OF status ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_email_outbox();

-- Email queue-closed ke owner saat antrian ditutup. Isi fungsi sama dengan db/migrations/01g_triggers.sql.
CREATE OR REPLACE FUNCTION queues_email_outbox() RETURNS trigger AS $fn$
BEGIN
    -- Hanya penutupan oleh owner (active menjadi closed) yang menghasilkan email.
    IF NOT (OLD.status = 'active' AND NEW.status = 'closed') THEN
        RETURN NULL;
    END IF;

    -- Sakelar pengguna dibaca saat kejadian, sama seperti email tiket. Tanpa tiket: ticket_id NULL, number 0.
    -- Satu antrian paling banyak satu email berkat indeks unik parsial (kind, queue_id).
    INSERT INTO notification_outbox (kind, ticket_id, queue_id, user_id, payload)
    SELECT 'queue-closed', NULL, q.id, u.id, jsonb_build_object(
        'queue_id', q.id, 'queue_name', q.queue_name, 'prefix', q.ticket_prefix,
        'number', 0, 'name', u.username)
    FROM queues q JOIN users u ON u.id = NEW.owner_id
    WHERE q.id = NEW.id AND u.email_notifications
    ON CONFLICT (kind, queue_id) WHERE queue_id IS NOT NULL DO NOTHING;

    IF FOUND THEN
        PERFORM pg_notify('mail_outbox', '');
    END IF;
    RETURN NULL;
END;
$fn$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS queues_email_outbox_trg ON queues;
CREATE TRIGGER queues_email_outbox_trg
    AFTER UPDATE OF status ON queues
    FOR EACH ROW EXECUTE FUNCTION queues_email_outbox();

-- Layar publik antrian (fitur 4.5): lihat db/migrations/01b_queues.sql.
CREATE TABLE IF NOT EXISTS queue_displays (
    queue_id       BIGINT PRIMARY KEY REFERENCES queues(id) ON DELETE CASCADE,
    token          TEXT NOT NULL UNIQUE,
    upcoming_count INT NOT NULL DEFAULT 5 CHECK (upcoming_count BETWEEN 0 AND 10),
    skipped_count  INT NOT NULL DEFAULT 0 CHECK (skipped_count BETWEEN 0 AND 10),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at     TIMESTAMPTZ
);

-- Hak role mail_rw (dibuat oleh 04_mail_role.sh): hanya yang dibutuhkan pengirim email.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mail_rw') THEN
        GRANT USAGE ON SCHEMA public TO mail_rw;
        GRANT SELECT ON notification_outbox TO mail_rw;
        GRANT UPDATE (sent_at, attempts, last_error, next_attempt_at, failed_at) ON notification_outbox TO mail_rw;
        GRANT SELECT (id, email, username, email_notifications) ON users TO mail_rw;
    END IF;
END $$;

-- Reset password (item 9): lihat db/migrations/01a_accounts.sql dan 01f_profile_security.sql.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS user_action_tokens (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL CHECK (purpose IN ('password_reset', 'email_verify', 'email_change')),
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    request_ip  TEXT
);
CREATE INDEX IF NOT EXISTS user_action_tokens_user_idx ON user_action_tokens (user_id, purpose, created_at DESC);

-- Verifikasi email: tabel yang sudah ada tidak mengubah CHECK-nya lewat CREATE TABLE IF NOT EXISTS,
-- jadi constraint diganti agar menerima purpose 'email_verify'. Aman dijalankan berulang.
ALTER TABLE user_action_tokens DROP CONSTRAINT IF EXISTS user_action_tokens_purpose_check;
ALTER TABLE user_action_tokens
    ADD CONSTRAINT user_action_tokens_purpose_check CHECK (purpose IN ('password_reset', 'email_verify', 'email_change'));

-- Kolom email_verified_at dan backfill dilakukan SEKALI, saat kolom belum ada. Semua akun yang sudah ada dianggap
-- terverifikasi agar tidak terkunci; menjalankan skrip ini lagi tidak memverifikasi akun baru yang belum terverifikasi.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = 'users' AND column_name = 'email_verified_at'
    ) THEN
        ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
        UPDATE users SET email_verified_at = created_at;
    END IF;
END $$;

-- Kode join 6 karakter (ganti token panjang lama). Kode lama yang panjangnya bukan 6 dikosongkan supaya GET QR
-- menerbitkan kode baru. Kode statis yang sudah dicetak tidak berlaku lagi. Kolom masa berlaku untuk kode dinamis.
ALTER TABLE queue_settings ADD COLUMN IF NOT EXISTS qr_expires_at TIMESTAMPTZ;
UPDATE queue_settings SET qr_current_code = NULL, qr_expires_at = NULL
 WHERE qr_current_code IS NOT NULL AND length(qr_current_code) <> 6;
CREATE UNIQUE INDEX IF NOT EXISTS queue_settings_qr_current_code_key
    ON queue_settings (qr_current_code) WHERE qr_current_code IS NOT NULL;

-- Pesanan pembayaran BorderPay (payment-service). Aman dijalankan berulang.
CREATE TABLE IF NOT EXISTS payment_orders (
    reference_id   TEXT PRIMARY KEY,
    owner_id       BIGINT NOT NULL REFERENCES users(id),
    amount         BIGINT NOT NULL CHECK (amount > 0),
    plan_type      VARCHAR NOT NULL CHECK (plan_type IN ('monthly', 'yearly', 'one_time')),
    status         VARCHAR NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'expired', 'failed')),
    pay_url        TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at        TIMESTAMPTZ
);
-- FK pesanan dipindah dari owners ke users (calon owner belum punya baris owners). Aman dijalankan berulang.
ALTER TABLE payment_orders DROP CONSTRAINT IF EXISTS payment_orders_owner_id_fkey;
ALTER TABLE payment_orders ADD CONSTRAINT payment_orders_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES users(id);

COMMIT;
