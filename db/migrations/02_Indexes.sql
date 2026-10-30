-- Index non-unik untuk lookup. Dijalankan setelah 01_Schema.sql.
-- Constraint PRIMARY KEY/UNIQUE tetap di 01_Schema.sql.

CREATE INDEX IF NOT EXISTS idx_payments_owner_paid_at
    ON payments (owner_id, paid_at);

-- Partial index: cuma indeks tiket yang masih 'waiting', karena itu satu-
-- satunya status yang di-query berulang kali per detik (status page &
-- public display, NFR #4). Tiket completed/cancelled/skipped tetap ada di
-- tabel untuk histori, tapi jarang dibaca dan tidak perlu ikut membengkakkan
-- index yang sering dipakai ini.
CREATE INDEX IF NOT EXISTS idx_tickets_queue_waiting_sort_key
    ON tickets (queue_id, sort_key) WHERE status = 'waiting';

-- Dipakai Go buat 2 hal: (1) list history per owner urut terbaru,
-- (2) hitung COUNT(DISTINCT original_queue_id) per owner buat cek "sudah
-- lewat 10 belum" sebelum decide perlu DELETE atau tidak.
CREATE INDEX IF NOT EXISTS idx_queue_history_owner_closed
    ON queue_history (owner_id, queue_closed_at);
