-- ============================================================================
-- 01_Schema.sql — Queue Management System
-- Versi final, mencakup semua revisi sepanjang pengembangan:
--   - Google Authentication (users.google_id)
--   - Notes edit-in-place (ticket_notes.ticket_id sebagai PK)
--   - Gap-based sort_key untuk urutan tiket (fractional indexing)
--   - Customer wajib login (tickets.user_id, bukan device_token)
--   - Owner subscription & payment (owners.subscription_expires_at + payments)
--   - Registration limit per-instance queue (bukan daily limit)
--   - History sebagai arsip terpisah (queue_history), tanpa FK ke tabel live
--   - ON DELETE CASCADE di semua FK anak queues/tickets
-- ============================================================================

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    username      VARCHAR NOT NULL,
    password_hash TEXT,                 -- nullable: akun Google-only tidak punya password
    google_id     TEXT UNIQUE,          -- ID akun Google; login/matching pakai ini, bukan email (FR 4.17)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- tiap user minimal punya satu cara login
    CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL)
);

-- Shared primary key / Class Table Inheritance: owners itu 1:1 dengan users,
-- jadi user_id LANGSUNG jadi PK (bukan id surrogate + user_id FK terpisah) —
-- pola yang sama dengan queue_settings/ticket_notes di bawah.
-- Ini juga yang mencegah satu user terdaftar sebagai owner lebih dari sekali.
CREATE TABLE owners (
    user_id                  BIGINT PRIMARY KEY REFERENCES users(id),

    -- Kapan masa subscription owner ini berakhir. NULL = belum pernah bayar.
    -- "Aktif atau tidak" TIDAK disimpan sebagai boolean tersendiri — dihitung
    -- saat dicek (subscription_expires_at > now()), supaya tidak butuh cron
    -- job tengah malam yang bisa telat/gagal.
    subscription_expires_at  TIMESTAMPTZ NOT NULL,

    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Riwayat transaksi pembayaran subscription (mirip Stripe Invoice/Charge).
-- owners.subscription_expires_at cuma nyimpen tanggal aktif SEKARANG;
-- tabel ini nyimpen jejak tiap pembayaran yang membentuk tanggal itu.
CREATE TABLE payments (
    id                       BIGSERIAL PRIMARY KEY,
    owner_id                 BIGINT NOT NULL REFERENCES owners(user_id),

    amount                   NUMERIC(12,2) NOT NULL,
    currency                 VARCHAR NOT NULL DEFAULT 'IDR',
    plan_type                VARCHAR NOT NULL
        CHECK (plan_type IN ('monthly', 'yearly', 'one_time')),

    extends_subscription_to  TIMESTAMPTZ NOT NULL,

    status                   VARCHAR NOT NULL DEFAULT 'success'
        CHECK (status IN ('success', 'failed', 'refunded')),
    paid_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE queues (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT NOT NULL REFERENCES owners(user_id),
    queue_name TEXT NOT NULL,
    queue_desc TEXT,
    -- Prefix nomor tiket yang dilihat customer (mis. 'A' -> A-001). 1-3 huruf/angka besar.
    ticket_prefix VARCHAR(3) NOT NULL DEFAULT 'A' CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$'),
    status     VARCHAR NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at  TIMESTAMPTZ
);

CREATE TABLE queue_settings (
    queue_id               BIGINT PRIMARY KEY REFERENCES queues(id) ON DELETE CASCADE,

    qr_type                VARCHAR NOT NULL DEFAULT 'static'
        CHECK (qr_type IN ('static', 'dynamic')),
    qr_current_code        TEXT,

    -- Batas jumlah customer yang boleh MENDAFTAR (bukan jumlah scan mentah)
    -- selama queue ini AKTIF — dari dibuat sampai closed_at, bukan per hari
    -- kalender. Tidak ada kolom "usage" tersendiri — pemakaiannya dihitung
    -- langsung dari COUNT(*) tabel tickets (status != 'cancelled') saat
    -- dibutuhkan, bukan disimpan & di-increment manual. NULL = tanpa batas.
    registration_limit     INT,

    display_next_count     INT NOT NULL DEFAULT 3,
    display_skipped_count  INT NOT NULL DEFAULT 3,

    -- Extends the static QR flow so the code can rotate per customer and be
    -- capped ke jumlah join PER HARI KALENDER — independen dari
    -- registration_limit di atas (yang berlaku seumur hidup queue instance).
    fallback_mode          VARCHAR NOT NULL DEFAULT 'none'
        CHECK (fallback_mode IN ('none', 'once', 'max_n', 'unlimited')),
    fallback_offset        INT,
    fallback_max_attempts  INT,

    -- Pengaturan skip: apa yang terjadi pada nomor yang dilewati owner.
    --   remove  : jadi 'skipped' dan tidak kembali
    --   back    : kembali 'waiting' di posisi paling belakang
    --   after_n : kembali 'waiting' setelah N tiket yang sedang menunggu (skip_fallback_n)
    -- skip_max_returns = berapa kali satu tiket boleh dikembalikan; sesudah itu 'skipped' permanen.
    skip_fallback          VARCHAR NOT NULL DEFAULT 'remove'
        CHECK (skip_fallback IN ('remove', 'back', 'after_n')),
    skip_fallback_n        INT CHECK (skip_fallback_n IS NULL OR skip_fallback_n >= 1),
    skip_max_returns       INT NOT NULL DEFAULT 1 CHECK (skip_max_returns >= 1),

    -- kapan terakhir kali sort_key tiket di queue ini dinormalkan ulang
    -- (rebalance) — murni observability/debug, tidak dipakai logika bisnis
    last_rebalanced_at     TIMESTAMPTZ,

    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- counters dideklarasikan sebelum tickets karena tickets.called_counter_id
-- mereferensikannya. Counter tidak menyimpan admin atau tiket yang sedang
-- dilayani: siapa pun yang login sebagai owner boleh memakai counter mana pun,
-- dan tiket sendiri yang mencatat counter pemanggilnya.
CREATE TABLE counters (
    id         BIGSERIAL PRIMARY KEY,
    queue_id   BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_default BOOLEAN NOT NULL DEFAULT FALSE
);

ALTER TABLE counters ADD CONSTRAINT counters_queue_id_name_key UNIQUE (queue_id, name);

CREATE TABLE tickets (
    id               BIGSERIAL PRIMARY KEY,
    queue_id         BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,

    ticket_number    INT NOT NULL,      -- nomor tetap yang dilihat customer, tidak pernah berubah

    -- Kunci urutan gap-based (fractional indexing ala Figma/Jira LexoRank):
    -- diberi jarak besar saat insert (kelipatan 65536) supaya requeue/
    -- fallback (FR 4.14) cukup menyisipkan 1 nilai baru di antara dua
    -- sort_key yang sudah ada — TANPA menggeser/menulis ulang baris tiket
    -- lain. "Posisi ke berapa" yang ditampilkan ke user dihitung saat baca
    -- lewat ROW_NUMBER() OVER (PARTITION BY queue_id, status ORDER BY
    -- sort_key), bukan disimpan sebagai angka urut yang harus disinkron
    -- manual.
    sort_key         BIGINT NOT NULL,

    -- Customer WAJIB login (revisi FR 4.2 — sebelumnya anonim lewat
    -- device_token). Identitas & anti-double-join sekarang berbasis akun
    -- (UNIQUE queue_id+user_id di bawah), bukan device/browser — ini lebih
    -- kuat karena tidak bisa dihindari dengan clear storage/incognito.
    user_id          BIGINT NOT NULL REFERENCES users(id),

    -- currently_served: nomor yang sedang dilayani (sumber kebenaran, bukan turunan dari completed).
    -- deleted: disiapkan untuk owner delete number; belum ditulis kode mana pun.
    status           VARCHAR NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('waiting', 'currently_served', 'completed', 'cancelled', 'skipped', 'deleted')),
    skip_count       INT NOT NULL DEFAULT 0,

    -- counter yang memanggil tiket ini (diisi saat Next); NULL selama belum dipanggil
    called_counter_id BIGINT REFERENCES counters(id) ON DELETE SET NULL,
    called_at         TIMESTAMPTZ,

    -- panggil ulang oleh owner: boleh sekali per pemanggilan (di-NULL-kan tiap tiket dipanggil)
    recalled_at      TIMESTAMPTZ,

    last_notified_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,
    cancelled_at     TIMESTAMPTZ,

    UNIQUE (queue_id, ticket_number),
    -- jaring pengaman: dua tiket di queue yang sama tidak boleh punya
    -- sort_key yang persis sama (mencegah race condition insert diam-diam korup)
    UNIQUE (queue_id, sort_key)
);

-- anti-double-join: 1 akun cuma bisa punya 1 tiket aktif per queue. Hanya tiket waiting dan
-- currently_served yang dihitung; setelah selesai/dibatalkan, akun boleh join lagi dan mendapat nomor baru.
CREATE UNIQUE INDEX tickets_queue_id_user_id_active_key
    ON tickets (queue_id, user_id)
    WHERE status IN ('waiting', 'currently_served');

-- Multi-counter: tiap counter melayani paling banyak satu nomor, dan nomor yang sedang dilayani selalu
-- punya counter. Dua perangkat di counter yang sama tidak bisa membuat dua nomor dilayani bersamaan.
CREATE UNIQUE INDEX tickets_one_serving_per_counter
    ON tickets (called_counter_id)
    WHERE status = 'currently_served';
ALTER TABLE tickets ADD CONSTRAINT tickets_serving_has_counter
    CHECK (status <> 'currently_served' OR called_counter_id IS NOT NULL);

-- Satu note per tiket: ticket_id sebagai PK, bukan surrogate id + FK biasa.
-- Menyimpan note kedua untuk tiket yang sama berarti UPDATE baris ini
-- (mengedit note & updated_at/updated_by), bukan INSERT baris baru (append).
CREATE TABLE ticket_notes (
    ticket_id  BIGINT PRIMARY KEY REFERENCES tickets(id) ON DELETE CASCADE,
    note       TEXT NOT NULL,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================================
-- Arsip History (FR 4.15) — tabel KHUSUS, terpisah dari queues/tickets yang
-- masih operasional (live). Sengaja TIDAK punya FK ke queues(id)/tickets(id):
--
-- 1. Queue AKTIF tidak akan pernah muncul di sini secara struktural — bukan
--    cuma difilter WHERE status='closed' seperti sebelumnya. Baris di sini
--    HANYA ada kalau Go sengaja menulisnya (satu kali, saat queue ditutup).
-- 2. Karena tidak ada FK balik ke queues/tickets, Go bebas nanti membersihkan
--    baris live (queues/tickets) tanpa mempengaruhi arsip ini sama sekali —
--    dua "wilayah" data ini sengaja diputus total, mirip pola "archive table"
--    yang lazim dipakai buat pisahkan data operasional (butuh cepat & selalu
--    konsisten, NFR #2) dari data historis (boleh eventually consistent).
-- 3. TIDAK ada trigger/function penghapusan di sini. Retensi "simpan 10
--    terbaru per owner" dan generate CSV/Excel keduanya murni logika Go:
--    Go INSERT ke sini saat queue ditutup, lalu Go sendiri yang menghitung
--    dan menjalankan DELETE kalau sudah lewat 10. Denormalized (1 baris =
--    1 tiket, sudah termasuk nama customer & note) supaya endpoint export
--    tinggal SELECT langsung tanpa JOIN.
-- ============================================================================
CREATE TABLE queue_history (
    id                  BIGSERIAL PRIMARY KEY,
    owner_id            BIGINT NOT NULL REFERENCES owners(user_id),

    -- id queue ASLI waktu masih live — cuma buat referensi/debug, BUKAN FK
    -- (queue aslinya boleh saja sudah dihapus Go dari tabel live nanti)
    original_queue_id   BIGINT NOT NULL,
    queue_name          TEXT NOT NULL,
    queue_opened_at     TIMESTAMPTZ NOT NULL,
    queue_closed_at     TIMESTAMPTZ NOT NULL,

    ticket_number       INT NOT NULL,
    customer_name       TEXT NOT NULL,
    customer_email      TEXT NOT NULL,
    outcome             VARCHAR NOT NULL,  -- snapshot tickets.status waktu diarsipkan
    joined_at           TIMESTAMPTZ NOT NULL,
    completed_at        TIMESTAMPTZ,
    cancelled_at        TIMESTAMPTZ,
    skip_count          INT NOT NULL DEFAULT 0,
    note                TEXT,

    archived_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Langganan Web Push per browser (satu user bisa punya banyak perangkat).
CREATE TABLE push_subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_push_subscriptions_user ON push_subscriptions(user_id);

-- Sinyal untuk notify-service (LISTEN queue_events). Hanya memancarkan; pengiriman push terjadi di luar
-- transaksi Next/Skip. NOTIFY dikirim saat COMMIT, jadi transaksi yang dibatalkan tidak mengirim apa pun.
CREATE FUNCTION tickets_notify() RETURNS trigger AS $$
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
    -- Transisi sementara: 'completed' dibiarkan selama masa transisi (lihat apply_schema.sql).
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
$$ LANGUAGE plpgsql;

CREATE TRIGGER tickets_notify_trg
    AFTER UPDATE OF status, called_at, recalled_at, skip_count ON tickets
    FOR EACH ROW EXECUTE FUNCTION tickets_notify();

-- Foto profil: JPEG 512x512 hasil olahan server, satu per user. Disimpan di database supaya ikut backup.
-- Batas 150 KB dijaga CHECK; notify_ro sengaja tidak diberi hak apa pun di tabel ini.
CREATE TABLE user_avatars (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    content_type TEXT NOT NULL DEFAULT 'image/jpeg',
    data         BYTEA NOT NULL CHECK (octet_length(data) <= 153600),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

