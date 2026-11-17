-- ============================================================================
-- 01c_tickets.sql — Tiket
-- Isi      : tickets, ticket_notes
-- Bergantung: 01a (users), 01b (queues, counters)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

CREATE TABLE tickets (
    id                 BIGSERIAL PRIMARY KEY,
    queue_id           BIGINT NOT NULL REFERENCES queues(id) ON DELETE CASCADE,

    -- Nomor yang dilihat customer; tetap, tidak pernah berubah.
    ticket_number      INT NOT NULL,

    -- Kunci urutan gap-based (fractional indexing ala Figma/Jira LexoRank): diberi jarak besar saat insert
    -- (kelipatan 65536) supaya requeue/fallback (FR 4.14) cukup menyisipkan satu nilai baru di antara dua
    -- sort_key yang ada, TANPA menulis ulang baris tiket lain. "Posisi ke berapa" dihitung saat baca lewat
    -- COUNT(*) tiket berstatus sama dengan sort_key lebih kecil di antrian yang sama, bukan disimpan.
    sort_key           BIGINT NOT NULL,

    -- Customer wajib login (revisi FR 4.2; sebelumnya anonim lewat device_token). Anti-double-join berbasis akun
    -- (index tickets_queue_id_user_id_active_key di bawah), jadi tidak bisa dihindari dengan clear storage/incognito.
    user_id            BIGINT NOT NULL REFERENCES users(id),

    -- waiting          : menunggu giliran
    -- currently_served : sedang dilayani (sumber kebenaran, bukan turunan dari completed)
    -- completed        : selesai dilayani
    -- cancelled        : dibatalkan customer
    -- skipped          : dilewati owner dan tidak kembali
    -- deleted          : dihapus owner (tidak ikut urutan; email "removed" dikirim lewat outbox)
    status             VARCHAR NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('waiting', 'currently_served', 'completed', 'cancelled', 'skipped', 'deleted')),
    skip_count         INT NOT NULL DEFAULT 0,

    -- Counter yang memanggil tiket ini (diisi saat Next); NULL selama belum dipanggil atau counter-nya dihapus.
    called_counter_id  BIGINT REFERENCES counters(id) ON DELETE SET NULL,
    called_at          TIMESTAMPTZ,
    -- Panggil ulang oleh owner: boleh sekali per pemanggilan (di-NULL-kan tiap tiket dipanggil).
    recalled_at        TIMESTAMPTZ,

    last_notified_at   TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at       TIMESTAMPTZ,
    cancelled_at       TIMESTAMPTZ,

    UNIQUE (queue_id, ticket_number),
    -- Jaring pengaman: dua tiket di antrian yang sama tidak boleh punya sort_key yang persis sama
    -- (mencegah race condition insert merusak urutan secara diam-diam).
    UNIQUE (queue_id, sort_key),

    -- Nomor yang sedang dilayani selalu punya counter.
    CONSTRAINT tickets_serving_has_counter
        CHECK (status <> 'currently_served' OR called_counter_id IS NOT NULL)
);

-- Anti-double-join: satu akun hanya punya satu tiket aktif per antrian. Hanya waiting dan currently_served
-- yang dihitung; setelah selesai atau dibatalkan, akun boleh join lagi dan mendapat nomor baru.
CREATE UNIQUE INDEX tickets_queue_id_user_id_active_key
    ON tickets (queue_id, user_id)
    WHERE status IN ('waiting', 'currently_served');

-- Multi-counter: tiap counter melayani paling banyak satu nomor. Dua perangkat di counter yang sama tidak bisa
-- membuat dua nomor dilayani bersamaan.
CREATE UNIQUE INDEX tickets_one_serving_per_counter
    ON tickets (called_counter_id)
    WHERE status = 'currently_served';

-- Catatan owner untuk satu tiket. ticket_id menjadi PK (bukan id surrogate), jadi menulis catatan kedua berarti
-- UPDATE baris ini (mengedit catatan dan updated_at/updated_by), bukan INSERT baris baru.
CREATE TABLE ticket_notes (
    ticket_id   BIGINT PRIMARY KEY REFERENCES tickets(id) ON DELETE CASCADE,
    note        TEXT NOT NULL,
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  BIGINT REFERENCES users(id),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
