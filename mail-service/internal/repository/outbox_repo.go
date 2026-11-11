package repository

import (
	"context"
	"fmt"
	"time"

	"qms/mail-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxRepository: akses tabel notification_outbox di atas Postgres, memakai role mail_rw (baca outbox, ubah status kirim, baca email pengguna).
type OutboxRepository struct {
	// pool: pool koneksi ke PostgreSQL.
	pool *pgxpool.Pool
}

// NewOutboxRepository membuat OutboxRepository dari pool koneksi yang sudah dibuka.
func NewOutboxRepository(
	pool *pgxpool.Pool,
) *OutboxRepository {
	// simpan pool koneksi untuk dipakai semua query
	return &OutboxRepository{pool: pool}
}

// claimSQL: klaim baris yang jatuh tempo. FOR UPDATE SKIP LOCKED membuat worker lain melewati baris yang sedang
// diklaim. attempts dinaikkan dan next_attempt_at digeser sebesar lease, supaya baris tidak diambil ulang
// sampai lease habis.
const claimSQL = `
WITH due AS (
    SELECT id FROM notification_outbox
    WHERE sent_at IS NULL AND failed_at IS NULL AND next_attempt_at <= now()
    ORDER BY next_attempt_at, id
    LIMIT $1
    FOR UPDATE SKIP LOCKED
), upd AS (
    UPDATE notification_outbox o
    SET attempts = o.attempts + 1, next_attempt_at = now() + make_interval(secs => $2)
    FROM due WHERE o.id = due.id
    RETURNING o.id, o.kind, o.user_id, o.attempts, o.payload
)
SELECT upd.id, upd.kind, u.email, upd.attempts, upd.payload
FROM upd JOIN users u ON u.id = upd.user_id
ORDER BY upd.id`

// Claim mengklaim paling banyak limit baris yang jatuh tempo dan menyewa baris itu selama lease.
func (s *OutboxRepository) Claim(
	ctx context.Context,
	limit int,
	lease time.Duration,
) ([]model.OutboxItem, error) {
	// Jalankan query klaim dengan batas jumlah baris dan lama sewa dalam detik.
	rows, err := s.pool.Query(ctx, claimSQL, limit, lease.Seconds())
	// Query gagal: bungkus galatnya.
	if err != nil {
		// Tidak ada baris yang diklaim bila query gagal.
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	// Tutup hasil query setelah fungsi selesai.
	defer rows.Close()
	// Kumpulkan baris yang diklaim.
	var out []model.OutboxItem
	// Baca hasil baris demi baris.
	for rows.Next() {
		// Siapkan item untuk baris ini.
		var it model.OutboxItem
		// Salin kolom ke item. Galat pemindaian dibungkus dan menghentikan pembacaan.
		if err := rows.Scan(&it.ID, &it.Kind, &it.Email, &it.Attempts, &it.Payload); err != nil {
			// Hentikan pembacaan bila salah satu baris tidak bisa dipindai.
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		// Tambahkan item ke hasil.
		out = append(out, it)
	}
	// Kembalikan hasil beserta galat yang terjadi saat membaca baris.
	return out, rows.Err()
}

// MarkSent menandai baris terkirim, dan menghapus galat lama.
func (s *OutboxRepository) MarkSent(
	ctx context.Context,
	id int64,
) error {
	// Isi sent_at dengan waktu sekarang dan kosongkan last_error.
	_, err := s.pool.Exec(ctx, `UPDATE notification_outbox SET sent_at = now(), last_error = NULL WHERE id = $1`, id)
	return err
}

// MarkRetry menyimpan galat terakhir dan menjadwalkan percobaan berikutnya setelah delay.
func (s *OutboxRepository) MarkRetry(
	ctx context.Context,
	id int64,
	cause string,
	delay time.Duration,
) error {
	// Simpan penyebab (dipotong ke 500 byte) dan geser jadwal percobaan sebesar delay.
	_, err := s.pool.Exec(ctx,
		`UPDATE notification_outbox SET last_error = $2, next_attempt_at = now() + make_interval(secs => $3) WHERE id = $1`,
		id, truncate(cause), delay.Seconds())
	return err
}

// MarkFailed menandai baris gagal permanen dan menyimpan penyebabnya (dipotong ke 500 byte).
func (s *OutboxRepository) MarkFailed(
	ctx context.Context,
	id int64,
	cause string,
) error {
	// Isi failed_at dengan waktu sekarang dan simpan penyebab.
	_, err := s.pool.Exec(ctx, `UPDATE notification_outbox SET failed_at = now(), last_error = $2 WHERE id = $1`, id, truncate(cause))
	return err
}

// truncate memotong teks ke 500 byte agar kolom last_error tidak membengkak. Galat SMTP bisa panjang dan
// tidak ada gunanya menyimpan seluruhnya.
func truncate(s string) string {
	// Teks lebih dari 500 byte: potong. Bisa memotong di tengah karakter UTF-8, tetapi last_error hanya untuk dibaca operator.
	if len(s) > 500 {
		return s[:500]
	}
	// Teks sudah cukup pendek.
	return s
}
