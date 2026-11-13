-- ============================================================================
-- Isi      : payment_orders (pesanan checkout BorderPay sebelum dan sesudah dibayar)
-- Bergantung: 01a (owners)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

CREATE TABLE payment_orders (
    -- ID pesanan dari backend utama. Dipakai sebagai reference_id di BorderPay dan di webhook.
    reference_id   TEXT PRIMARY KEY,
    -- Mereferensi users, bukan owners: calon owner belum punya baris owners sampai pembayaran lunas.
    owner_id       BIGINT NOT NULL REFERENCES users(id),
    -- Nominal dalam rupiah; webhook harus membawa nominal yang sama.
    amount         BIGINT NOT NULL CHECK (amount > 0),
    plan_type      VARCHAR NOT NULL CHECK (plan_type IN ('monthly', 'yearly', 'one_time')),
    status         VARCHAR NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'expired', 'failed')),
    -- URL halaman pembayaran BorderPay (pay_url), diisi setelah BorderPay membalas checkout.
    pay_url        TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at        TIMESTAMPTZ
);
