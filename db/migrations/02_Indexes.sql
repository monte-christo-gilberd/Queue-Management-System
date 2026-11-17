-- ============================================================================
-- 02_Indexes.sql — index non-unik untuk query lookup
-- ============================================================================
-- Dijalankan setelah semua 01*.sql (Docker menjalankannya otomatis di database kosong).
--
-- Pembagian tugas dengan 01*.sql (urutan dan aturan: lihat README.md di folder ini):
--   * 01*.sql memuat constraint PRIMARY KEY dan UNIQUE, serta dua index unik parsial yang menegakkan aturan
--     bisnis (tickets_queue_id_user_id_active_key, tickets_one_serving_per_counter) di 01c_tickets.sql.
--   * File ini memuat SEMUA index lain, yaitu yang hanya mempercepat query. Index tiap tabel dikelompokkan
--     di bawah satu judul, dengan keterangan query yang dilayaninya.
--
-- Aman dijalankan berulang (IF NOT EXISTS). Nama index tidak boleh diganti tanpa migrasi: database yang
-- sudah berjalan dan db/scripts/apply_schema.sql memakai nama yang sama.
-- ============================================================================


-- ----------------------------------------------------------------------------
-- payments
-- ----------------------------------------------------------------------------

-- Riwayat pembayaran satu owner, berurutan menurut waktu.
CREATE INDEX IF NOT EXISTS idx_payments_owner_paid_at
    ON payments (owner_id, paid_at);


-- ----------------------------------------------------------------------------
-- queues
-- ----------------------------------------------------------------------------

-- Daftar antrian satu owner, terbaru dulu.
CREATE INDEX IF NOT EXISTS idx_queues_owner_created_at
    ON queues (owner_id, created_at DESC);


-- ----------------------------------------------------------------------------
-- tickets
-- ----------------------------------------------------------------------------

-- Tiket yang menunggu, berurutan menurut sort_key. Partial: hanya status 'waiting', karena itu satu-satunya
-- status yang di-query berulang kali per detik (halaman status dan layar publik, NFR #4). Tiket
-- completed/cancelled/skipped tetap ada untuk histori, tetapi jarang dibaca dan tidak perlu membengkakkan
-- index yang sering dipakai ini.
CREATE INDEX IF NOT EXISTS idx_tickets_queue_waiting_sort_key
    ON tickets (queue_id, sort_key) WHERE status = 'waiting';

-- Nomor yang terakhir selesai dilayani (LastCompletedNumber, RecentCompletedNumbers). Partial: hanya
-- 'completed', terbaru dulu.
CREATE INDEX IF NOT EXISTS idx_tickets_queue_completed
    ON tickets (queue_id, completed_at DESC, id DESC) WHERE status = 'completed';

-- Riwayat tiket satu customer (ListByUser), terbaru dulu.
CREATE INDEX IF NOT EXISTS idx_tickets_user_created_at
    ON tickets (user_id, created_at DESC);


-- ----------------------------------------------------------------------------
-- queue_history
-- ----------------------------------------------------------------------------

-- Disiapkan untuk dua hal (belum dipakai kode Go saat ini): (1) daftar riwayat satu owner, terbaru dulu; (2) COUNT(DISTINCT original_queue_id)
-- per owner, untuk memutuskan apakah sudah lewat 10 antrian dan perlu DELETE.
CREATE INDEX IF NOT EXISTS idx_queue_history_owner_closed
    ON queue_history (owner_id, queue_closed_at);


-- ----------------------------------------------------------------------------
-- push_subscriptions
-- ----------------------------------------------------------------------------

-- Semua langganan push satu user (kirim ke tiap perangkat, atau lepas saat logout).
CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user
    ON push_subscriptions (user_id);

-- ----------------------------------------------------------------------------
-- push_devices
-- ----------------------------------------------------------------------------

-- Semua token FCM satu user (kirim ke tiap perangkat Android).
CREATE INDEX IF NOT EXISTS idx_push_devices_user
    ON push_devices (user_id);


-- ----------------------------------------------------------------------------
-- notification_outbox
-- ----------------------------------------------------------------------------

-- Antrean kirim email: hanya baris yang belum terkirim dan belum gagal permanen, urut menurut jadwal coba.
-- Partial, jadi index tetap kecil walau tabel menumpuk baris yang sudah terkirim.
CREATE INDEX IF NOT EXISTS idx_outbox_pending
    ON notification_outbox (next_attempt_at)
    WHERE sent_at IS NULL AND failed_at IS NULL;


-- ----------------------------------------------------------------------------
-- user_action_tokens
-- ----------------------------------------------------------------------------

-- Token terbaru satu user per tujuan: batas kirim per jam (CountRecentTokens) dan pembatalan token lama.
-- (Namanya tidak berawalan idx_ karena sudah dipakai database yang berjalan; jangan diganti.)
CREATE INDEX IF NOT EXISTS user_action_tokens_user_idx
    ON user_action_tokens (user_id, purpose, created_at DESC);
