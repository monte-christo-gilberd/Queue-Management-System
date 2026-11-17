-- ============================================================================
-- 01a_accounts.sql — Akun dan langganan
-- Isi      : users, owners, payments
-- Bergantung: -
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

CREATE TABLE users (
    id                   BIGSERIAL PRIMARY KEY,
    email                TEXT NOT NULL UNIQUE,
    username             VARCHAR NOT NULL,       -- tidak unik; login menerima email atau username
    password_hash        TEXT,                   -- NULL untuk akun Google-only
    google_id            TEXT UNIQUE,            -- ID akun Google; login/matching pakai ini, bukan email (FR 4.17)
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Sakelar email notifikasi tiket. Dibaca trigger tickets_email_outbox saat kejadian.
    email_notifications  BOOLEAN NOT NULL DEFAULT TRUE,
    -- Sesi (JWT) yang diterbitkan sebelum waktu ini dicabut. Diisi saat password diganti atau direset.
    password_changed_at  TIMESTAMPTZ,
    -- Kapan pemilik membuktikan emailnya lewat tautan. NULL = belum terverifikasi.
    -- Untuk database lama, apply_schema.sql menambahkan kolom ini dan memverifikasi akun yang sudah ada sekali saja.
    email_verified_at    TIMESTAMPTZ,

    -- tiap user minimal punya satu cara login
    CHECK (password_hash IS NOT NULL OR google_id IS NOT NULL)
);

-- owners 1:1 dengan users (shared primary key), jadi user_id langsung menjadi PK. Pola yang sama dengan
-- queue_settings, ticket_notes, dan user_avatars. Ini juga mencegah satu user terdaftar sebagai owner dua kali.
CREATE TABLE owners (
    user_id                  BIGINT PRIMARY KEY REFERENCES users(id),

    -- Akhir masa langganan. "Aktif" TIDAK disimpan sebagai boolean; dihitung saat dicek
    -- (subscription_expires_at > now()), supaya tidak butuh cron yang bisa telat atau gagal.
    subscription_expires_at  TIMESTAMPTZ NOT NULL,

    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Jejak tiap pembayaran langganan. owners.subscription_expires_at hanya menyimpan tanggal aktif SEKARANG;
-- tabel ini menyimpan pembayaran yang membentuk tanggal itu. Ditulis oleh payment-service (role payment_rw) setelah pembayaran BorderPay berhasil; backend hanya memulai checkout
-- (POST /api/v1/owners/subscription/checkout).
CREATE TABLE payments (
    id                       BIGSERIAL PRIMARY KEY,
    owner_id                 BIGINT NOT NULL REFERENCES owners(user_id),

    amount                   NUMERIC(12,2) NOT NULL,
    currency                 VARCHAR NOT NULL DEFAULT 'IDR',
    -- Kode saat ini hanya menulis 'monthly'; 'yearly' dan 'one_time' disiapkan.
    plan_type                VARCHAR NOT NULL
        CHECK (plan_type IN ('monthly', 'yearly', 'one_time')),

    extends_subscription_to  TIMESTAMPTZ NOT NULL,   -- tanggal berakhir baru setelah pembayaran ini

    status                   VARCHAR NOT NULL DEFAULT 'success'
        CHECK (status IN ('success', 'failed', 'refunded')),
    paid_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
