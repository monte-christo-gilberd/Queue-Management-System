-- Backfill data lama: tiket completed terakhir (called_at terbaru) di antrian yang masih aktif
-- menjadi currently_served. Jalankan MANUAL setelah backup, dan setelah apply_schema.sql.
-- Tidak dijalankan otomatis dari apply_schema.sql.
--
-- Skrip ini berakhir dengan ROLLBACK. Periksa jumlah baris yang berubah, lalu ganti ROLLBACK menjadi COMMIT.
--   dev:  docker compose exec -T db psql -U qms -d qms_db -v ON_ERROR_STOP=1 < db/scripts/backfill_currently_served.sql
BEGIN;

UPDATE tickets t
SET status = 'currently_served', completed_at = NULL
WHERE t.status = 'completed' AND t.called_at IS NOT NULL
  AND t.id = (
      SELECT x.id FROM tickets x
      WHERE x.queue_id = t.queue_id
        AND x.status = 'completed'
        AND x.called_at IS NOT NULL
      ORDER BY x.called_at DESC, x.id DESC
      LIMIT 1
  )
  AND EXISTS (
      SELECT 1 FROM queues q
      WHERE q.id = t.queue_id AND q.status = 'active'
  );

-- Pastikan paling banyak satu tiket currently_served per antrian sebelum commit.
SELECT queue_id, COUNT(*) AS serving FROM tickets
WHERE status = 'currently_served'
GROUP BY queue_id
HAVING COUNT(*) > 1;

ROLLBACK;
