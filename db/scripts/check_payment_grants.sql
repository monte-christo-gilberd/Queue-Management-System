-- ============================================================================
-- Isi      : memeriksa bahwa role payment_rw punya semua hak yang dibutuhkan payment-service.
-- Sifat    : hanya membaca katalog (has_*_privilege). Tidak mengubah apa pun.
-- Dipakai  : make prod-payment-check, dan make prod-up setelah db/migrations/08_payment_role.sh.
-- Galat    : bila ada hak yang hilang, RAISE EXCEPTION menyebut semuanya sekaligus.
--
-- Sumber daftar hak: db/migrations/08_payment_role.sh. Bila hak di sana berubah, ubah juga di sini.
-- Latar belakang: database prod yang sudah ada tidak menerima ulang 08_payment_role.sh (initdb hanya jalan
-- saat volume kosong), sehingga grant yang ditambahkan kemudian bisa hilang tanpa terdeteksi. Akibatnya
-- webhook BorderPay gagal 500 dengan "permission denied for table owners".
-- ============================================================================

DO $$
DECLARE
    missing text[] := ARRAY[]::text[];
BEGIN
    -- Akses database dan schema.
    IF NOT has_database_privilege('payment_rw', current_database(), 'CONNECT') THEN
        missing := missing || 'CONNECT pada database';
    END IF;
    IF NOT has_schema_privilege('payment_rw', 'public', 'USAGE') THEN
        missing := missing || 'USAGE pada schema public';
    END IF;

    -- owners: baca user_id dan langganan; buat baris baru (calon owner) dan perbarui subscription_expires_at.
    -- INSERT ... ON CONFLICT DO UPDATE memeriksa INSERT lebih dulu, jadi owner yang sudah ada tetap butuh INSERT.
    IF NOT has_column_privilege('payment_rw', 'owners', 'user_id', 'SELECT') THEN
        missing := missing || 'owners.user_id SELECT';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'owners', 'subscription_expires_at', 'SELECT') THEN
        missing := missing || 'owners.subscription_expires_at SELECT';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'owners', 'user_id', 'INSERT') THEN
        missing := missing || 'owners.user_id INSERT';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'owners', 'subscription_expires_at', 'INSERT') THEN
        missing := missing || 'owners.subscription_expires_at INSERT';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'owners', 'subscription_expires_at', 'UPDATE') THEN
        missing := missing || 'owners.subscription_expires_at UPDATE';
    END IF;

    -- payment_orders: buat pesanan, simpan pay_url, dan tandai lunas.
    IF NOT has_table_privilege('payment_rw', 'payment_orders', 'SELECT') THEN
        missing := missing || 'payment_orders SELECT';
    END IF;
    IF NOT has_table_privilege('payment_rw', 'payment_orders', 'INSERT') THEN
        missing := missing || 'payment_orders INSERT';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'payment_orders', 'status', 'UPDATE') THEN
        missing := missing || 'payment_orders.status UPDATE';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'payment_orders', 'pay_url', 'UPDATE') THEN
        missing := missing || 'payment_orders.pay_url UPDATE';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'payment_orders', 'paid_at', 'UPDATE') THEN
        missing := missing || 'payment_orders.paid_at UPDATE';
    END IF;
    IF NOT has_column_privilege('payment_rw', 'payment_orders', 'updated_at', 'UPDATE') THEN
        missing := missing || 'payment_orders.updated_at UPDATE';
    END IF;

    -- payments: catat pembayaran lunas (id memakai sequence).
    IF NOT has_table_privilege('payment_rw', 'payments', 'INSERT') THEN
        missing := missing || 'payments INSERT';
    END IF;
    IF NOT has_sequence_privilege('payment_rw', 'payments_id_seq', 'USAGE') THEN
        missing := missing || 'payments_id_seq USAGE';
    END IF;

    IF array_length(missing, 1) IS NOT NULL THEN
        RAISE EXCEPTION 'hak role payment_rw kurang: %. Jalankan ulang db/migrations/08_payment_role.sh (make prod-up).',
            array_to_string(missing, ', ');
    END IF;

    RAISE NOTICE 'hak role payment_rw lengkap';
END
$$;
