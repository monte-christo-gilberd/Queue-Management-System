-- Penyesuaian skema idempoten untuk DB yang SUDAH berjalan (tidak ada migration runner).
-- Aman dijalankan berulang, dan tidak menyentuh data. Untuk DB baru, 01_Schema.sql sudah lengkap.
--   dev:  docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
--   prod: docker compose -f docker-compose.prod.yml --env-file .env.prod exec -T db \
--           psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/apply_schema.sql
-- Setelah itu restart backend dan notify-service. Backup dulu di prod.
BEGIN;

ALTER TABLE tickets ADD COLUMN IF NOT EXISTS recalled_at TIMESTAMPTZ;
ALTER TABLE queues ADD COLUMN IF NOT EXISTS ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A';
ALTER TABLE queue_settings ADD COLUMN IF NOT EXISTS skip_fallback VARCHAR NOT NULL DEFAULT 'remove';
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

-- Status 'currently_served' (nomor yang sedang dilayani) menggantikan 'completed' sebagai sumber sinyal 'called'.
-- Kondisi 'completed' sengaja dibiarkan selama masa transisi: urutan deploy tidak penting.
-- Setelah service baru berjalan dan backfill terverifikasi, persempit menjadi = 'currently_served'.
CREATE OR REPLACE FUNCTION tickets_notify() RETURNS trigger AS $fn$
BEGIN
    IF NEW.skip_count > OLD.skip_count THEN
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

-- Status currently_served dan deleted (deleted belum dipakai kode; hanya disiapkan).
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
        GRANT SELECT ON tickets, queues, push_subscriptions TO notify_ro;
        GRANT DELETE ON push_subscriptions TO notify_ro;
    END IF;
END $$;

COMMIT;
