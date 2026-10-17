-- ============================================================================
-- 01_Schema.sql — Queue Management System
-- Versi final, mencakup semua revisi sepanjang pengembangan:
--   - Google Authentication (users.google_id)
--   - Otorisasi admin per-queue (queue_authz keyed by queue_id saja)
--   - Notes edit-in-place (ticket_notes.ticket_id sebagai PK)
--   - Gap-based sort_key untuk urutan tiket (fractional indexing)
--   - Customer wajib login (tickets.user_id, bukan device_token)
--   - Owner subscription & payment (owners.subscription_expires_at + payments)
--   - Registration limit per-instance queue (bukan daily limit)
--   - Roster admin tersimpan (owner_saved_admins)
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
-- pola yang sama dengan queue_settings/queue_authz/ticket_notes di bawah.
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

CREATE INDEX idx_payments_owner_paid_at
    ON payments (owner_id, paid_at);

CREATE TABLE queues (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT NOT NULL REFERENCES owners(user_id),
    queue_name TEXT NOT NULL,
    queue_desc TEXT,
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

    -- kapan terakhir kali sort_key tiket di queue ini dinormalkan ulang
    -- (rebalance) — murni observability/debug, tidak dipakai logika bisnis
    last_rebalanced_at     TIMESTAMPTZ,

    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One-to-one authz per queue (PK = queue_id, bukan queue_id+admin_id):
-- otorisasi berlaku sama untuk semua admin yang ditugaskan ke queue ini,
-- bukan didelegasikan berbeda-beda per admin. Ini keputusan desain yang
-- disengaja, lihat product-overview.md FR 4.6.
CREATE TABLE queue_authz (
    queue_id             BIGINT PRIMARY KEY REFERENCES queues(id) ON DELETE CASCADE,

    can_manage_queue     BOOLEAN NOT NULL DEFAULT false,
    can_manage_counters  BOOLEAN NOT NULL DEFAULT false,
    can_manage_qr        BOOLEAN NOT NULL DEFAULT false,
    can_view_queue       BOOLEAN NOT NULL DEFAULT true,
    can_manage_display   BOOLEAN NOT NULL DEFAULT false,
    can_continue_queue   BOOLEAN NOT NULL DEFAULT false,
    can_notify_customer  BOOLEAN NOT NULL DEFAULT false,
    can_manage_tickets   BOOLEAN NOT NULL DEFAULT false,
    can_manage_notes     BOOLEAN NOT NULL DEFAULT false
);

-- Admin mana saja yang ditugaskan ke queue mana (penugasan AKTIF)
CREATE TABLE queue_admins (
    queue_id   BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    admin_id   BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (queue_id, admin_id)
);

-- Roster admin yang DISIMPAN owner secara umum — beda dari queue_admins di
-- atas (yang mencatat penugasan AKTIF ke 1 queue tertentu). Ini supaya owner
-- tidak perlu cari/ketik ulang email orang yang sama tiap kali bikin queue
-- baru. Menyimpan seseorang di sini TIDAK otomatis memberi akses ke queue
-- apa pun — itu baru terjadi kalau juga ada baris di queue_admins.
CREATE TABLE owner_saved_admins (
    owner_id   BIGINT NOT NULL REFERENCES owners(user_id),
    admin_id   BIGINT NOT NULL REFERENCES users(id),
    label      TEXT,                 -- catatan owner sendiri, mis. "Kasir shift pagi"
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, admin_id)
);

-- tickets dideklarasikan sebelum counters karena counters mereferensikannya,
-- tapi didefinisikan lebih dulu di bawah agar forward reference tidak error.
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

    status           VARCHAR NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('waiting', 'completed', 'cancelled', 'skipped')),
    skip_count       INT NOT NULL DEFAULT 0,

    last_notified_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at     TIMESTAMPTZ,
    cancelled_at     TIMESTAMPTZ,

    UNIQUE (queue_id, ticket_number),
    -- anti-double-join: 1 akun cuma bisa punya 1 tiket aktif per queue
    UNIQUE (queue_id, user_id),
    -- jaring pengaman: dua tiket di queue yang sama tidak boleh punya
    -- sort_key yang persis sama (mencegah race condition insert diam-diam korup)
    UNIQUE (queue_id, sort_key)
);

-- Partial index: cuma indeks tiket yang masih 'waiting', karena itu satu-
-- satunya status yang di-query berulang kali per detik (status page &
-- public display, NFR #4). Tiket completed/cancelled/skipped tetap ada di
-- tabel untuk histori, tapi jarang dibaca dan tidak perlu ikut membengkakkan
-- index yang sering dipakai ini.
CREATE INDEX idx_tickets_queue_waiting_sort_key
    ON tickets (queue_id, sort_key) WHERE status = 'waiting';

CREATE TABLE counters (
    id                  BIGSERIAL PRIMARY KEY,
    queue_id            BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    attending_admin_id  BIGINT REFERENCES users(id),
    attended_ticket_id  BIGINT UNIQUE REFERENCES tickets(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

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

-- Dipakai Go buat 2 hal: (1) list history per owner urut terbaru,
-- (2) hitung COUNT(DISTINCT original_queue_id) per owner buat cek "sudah
-- lewat 10 belum" sebelum decide perlu DELETE atau tidak.
CREATE INDEX idx_queue_history_owner_closed
    ON queue_history (owner_id, queue_closed_at);
