-- ============================================================================
-- 01e_notifications.sql — Notifikasi
-- Isi      : push_subscriptions, notification_outbox
-- Bergantung: 01a (users), 01c (tickets)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

-- Langganan Web Push per browser (satu user bisa punya banyak perangkat). Dibaca notify-service (role notify_ro),
-- yang juga boleh menghapus langganan yang sudah kedaluwarsa.
CREATE TABLE push_subscriptions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint    TEXT NOT NULL UNIQUE,
    p256dh      TEXT NOT NULL,
    auth        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Outbox email ke pemilik tiket (item 8). Trigger tickets_email_outbox menulis baris di sini DALAM transaksi
-- yang sama dengan perubahan tiket, jadi email tidak hilang walau mail-service mati. Worker di mail-service
-- (role mail_rw) mengirimnya dan mencoba ulang. Terpisah dari push: push tidak bergantung pada email sama sekali.
CREATE TABLE notification_outbox (
    id               BIGSERIAL PRIMARY KEY,
    kind             TEXT NOT NULL CHECK (kind IN ('joined', 'cancelled', 'removed', 'completed')),
    ticket_id        BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload          JSONB NOT NULL,   -- snapshot: queue_id, queue_name, prefix, number, name (tanpa catatan owner)
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    next_attempt_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at          TIMESTAMPTZ,
    attempts         INT NOT NULL DEFAULT 0,
    last_error       TEXT,
    failed_at        TIMESTAMPTZ,      -- gagal permanen (5xx atau batas percobaan): tidak dicoba lagi

    UNIQUE (kind, ticket_id)           -- satu email per kejadian
);
