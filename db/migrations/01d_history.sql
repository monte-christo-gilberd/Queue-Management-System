-- ============================================================================
-- 01d_history.sql — Arsip riwayat
-- Isi      : queue_history
-- Bergantung: 01a (owners)
-- Urutan dan aturan seluruh file migrasi: lihat README.md di folder ini.
-- ============================================================================

-- CATATAN: saat ini tidak ada kode Go yang membaca atau menulis tabel ini (hanya seed_prod_penguji.sql yang mengisinya).
-- Riwayat yang tampil di aplikasi dibaca dari queues/tickets. Rancangan di bawah adalah rencana arsip, belum diterapkan.
-- Tabel KHUSUS, terpisah dari queues/tickets yang operasional. Sengaja TIDAK punya FK ke queues(id) atau
-- tickets(id):
--   1. Antrian aktif tidak pernah muncul di sini secara struktural. Baris hanya ada bila sengaja ditulis
--      (rencana: sekali, saat antrian ditutup).
--   2. Tanpa FK balik, baris live (queues/tickets) bebas dibersihkan tanpa memengaruhi arsip. Data operasional
--      (harus cepat dan selalu konsisten, NFR #2) sengaja dipisah dari data historis.
--   3. Tidak ada trigger penghapusan. Retensi "simpan 10 terbaru per owner" dan ekspor CSV/Excel direncanakan sebagai logika Go (belum ada).
-- Denormalisasi: satu baris = satu tiket, sudah termasuk nama customer dan catatan, supaya ekspor cukup SELECT
-- tanpa JOIN.

CREATE TABLE queue_history (
    id                 BIGSERIAL PRIMARY KEY,
    owner_id           BIGINT NOT NULL REFERENCES owners(user_id),

    -- ID antrian asli saat masih live. Hanya referensi/debug, BUKAN FK (antrian aslinya boleh sudah dihapus).
    original_queue_id  BIGINT NOT NULL,
    queue_name         TEXT NOT NULL,
    queue_opened_at    TIMESTAMPTZ NOT NULL,
    queue_closed_at    TIMESTAMPTZ NOT NULL,

    ticket_number      INT NOT NULL,
    customer_name      TEXT NOT NULL,
    customer_email     TEXT NOT NULL,
    outcome            VARCHAR NOT NULL,   -- snapshot tickets.status saat diarsipkan
    joined_at          TIMESTAMPTZ NOT NULL,
    completed_at       TIMESTAMPTZ,
    cancelled_at       TIMESTAMPTZ,
    skip_count         INT NOT NULL DEFAULT 0,
    note               TEXT,

    archived_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
