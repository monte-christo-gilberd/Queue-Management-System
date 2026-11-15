-- ============================================================================
-- 01b_queues.sql — Antrian
-- Isi      : queues, queue_settings, counters, queue_displays
-- Bergantung: 01a (owners)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

CREATE TABLE queues (
    id             BIGSERIAL PRIMARY KEY,
    owner_id       BIGINT NOT NULL REFERENCES owners(user_id),
    queue_name     TEXT NOT NULL,
    queue_desc     TEXT,
    -- Prefix nomor tiket yang dilihat customer (mis. 'A' -> A-001). 1-3 huruf besar atau angka.
    ticket_prefix  VARCHAR(3) NOT NULL DEFAULT 'A' CHECK (ticket_prefix ~ '^[A-Z0-9]{1,3}$'),
    status         VARCHAR NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'closed')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at      TIMESTAMPTZ
);

-- Pengaturan satu antrian (1:1 dengan queues). Kolom dikelompokkan menurut apakah kode memakainya.
CREATE TABLE queue_settings (
    queue_id               BIGINT PRIMARY KEY REFERENCES queues(id) ON DELETE CASCADE,

    -- [dipakai] Mode QR join: statis (kode tetap, bisa diganti owner) atau dinamis (kode berubah berkala).
    qr_type                VARCHAR NOT NULL DEFAULT 'static'
        CHECK (qr_type IN ('static', 'dynamic')),
    qr_current_code        TEXT,
    -- Kode join aktif (6 karakter). Unik di seluruh antrian karena pengunjung hanya mengetik kode, tanpa ID antrian.
    -- Kode dinamis punya masa berlaku di qr_expires_at; kode statis tidak (NULL).
    qr_expires_at          TIMESTAMPTZ,

    -- [tidak dipakai] Batas pendaftar per antrian. NULL = tanpa batas. Dihapus dari produk 31 Okt 2026.
    registration_limit     INT,

    -- [tidak dipakai] Jumlah nomor berikutnya/dilewati di layar publik. Digantikan queue_displays
    -- (upcoming_count dan skipped_count), yang disimpan per antrian di tabel tersendiri.
    display_next_count     INT NOT NULL DEFAULT 3,
    display_skipped_count  INT NOT NULL DEFAULT 3,

    -- [tidak dipakai] Rotasi kode QR per customer dan batas join per hari kalender. Dihapus dari produk
    -- 31 Okt 2026. Jangan dikacaukan dengan skip_fallback di bawah.
    fallback_mode          VARCHAR NOT NULL DEFAULT 'none'
        CHECK (fallback_mode IN ('none', 'once', 'max_n', 'unlimited')),
    fallback_offset        INT,
    fallback_max_attempts  INT,

    -- [dipakai] Pengaturan Skip: apa yang terjadi pada nomor yang dilewati owner.
    --   remove  : jadi 'skipped' dan tidak kembali
    --   back    : kembali 'waiting' di posisi paling belakang (bawaan antrian baru)
    --   after_n : kembali 'waiting' setelah N tiket yang sedang menunggu (skip_fallback_n)
    -- skip_max_returns = berapa kali satu tiket boleh dikembalikan; sesudah itu 'skipped' permanen.
    skip_fallback          VARCHAR NOT NULL DEFAULT 'back'
        CHECK (skip_fallback IN ('remove', 'back', 'after_n')),
    skip_fallback_n        INT CHECK (skip_fallback_n IS NULL OR skip_fallback_n >= 1),
    skip_max_returns       INT NOT NULL DEFAULT 1 CHECK (skip_max_returns >= 1),

    -- [tidak dipakai] Kapan sort_key tiket antrian ini terakhir dinormalkan ulang. Murni catatan debug.
    last_rebalanced_at     TIMESTAMPTZ,

    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Kode join aktif harus unik di seluruh antrian (lihat qr_current_code). Kode lama yang sudah diganti tidak disimpan.
CREATE UNIQUE INDEX queue_settings_qr_current_code_key ON queue_settings (qr_current_code) WHERE qr_current_code IS NOT NULL;

-- Loket pemanggil. Counter tidak menyimpan admin atau tiket yang sedang dilayani: siapa pun yang login sebagai
-- owner boleh memakai counter mana pun, dan tiketlah yang mencatat counter pemanggilnya (tickets.called_counter_id).
-- Dideklarasikan sebelum tickets karena tickets mereferensikannya.
CREATE TABLE counters (
    id          BIGSERIAL PRIMARY KEY,
    queue_id    BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,   -- counter default tidak bisa dihapus

    CONSTRAINT counters_queue_id_name_key UNIQUE (queue_id, name)
);

-- Layar publik antrian (fitur 4.5): satu baris per antrian yang layarnya diaktifkan. `token` acak 24 byte dipakai
-- di URL publik (/display/<token>); disimpan mentah karena owner perlu melihat URL-nya lagi. Siapa pun yang
-- memegang token bisa melihat nomor antrian itu, jadi notify_ro dan mail_rw sengaja tidak diberi hak di sini.
CREATE TABLE queue_displays (
    queue_id        BIGINT PRIMARY KEY REFERENCES queues(id) ON DELETE CASCADE,
    token           TEXT NOT NULL UNIQUE,
    upcoming_count  INT NOT NULL DEFAULT 5 CHECK (upcoming_count BETWEEN 0 AND 10),   -- nomor berikutnya yang ditampilkan
    skipped_count   INT NOT NULL DEFAULT 0 CHECK (skipped_count BETWEEN 0 AND 10),    -- nomor dilewati yang ditampilkan
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at      TIMESTAMPTZ                                                       -- terakhir token diganti
);
