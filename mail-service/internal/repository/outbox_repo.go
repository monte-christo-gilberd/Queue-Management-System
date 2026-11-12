package repository

import (
	"context"
	"fmt"
	"time"

	"qms/mail-service/internal/model"

	"github.com/jmoiron/sqlx"
)

// OutboxRepository: akses tabel notification_outbox di atas Postgres (sqlx dengan driver pgx), memakai role mail_rw (baca outbox, ubah status kirim, baca email pengguna).
type OutboxRepository struct {
	// db: pool koneksi ke PostgreSQL.
	db *sqlx.DB
}

// NewOutboxRepository membuat OutboxRepository dari pool koneksi yang sudah dibuka.
func NewOutboxRepository(
	db *sqlx.DB,
) *OutboxRepository {
	// simpan pool koneksi untuk dipakai semua query
	return &OutboxRepository{db: db}
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
	rows, err := s.db.QueryContext(
		ctx,
		claimSQL,
		limit,
		lease.Seconds(),
	)

	// Query gagal: bungkus errornya.
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
		// Payload (jsonb) dibaca ke []byte dulu, lalu dijadikan json.RawMessage.
		var payload []byte
		// Salin kolom ke item. error pemindaian dibungkus dan menghentikan pembacaan.
		if err := rows.Scan(
			&it.ID,
			&it.Kind,
			&it.Email,
			&it.Attempts,
			&payload,
		); err != nil {
			// Hentikan pembacaan bila salah satu baris tidak bisa dipindai.
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		it.Payload = payload
		// Tambahkan item ke hasil.
		out = append(out, it)
	}
	// Kembalikan hasil beserta error yang terjadi saat membaca baris.
	return out, rows.Err()
}

// MarkSent menandai baris terkirim, dan menghapus error lama.
func (s *OutboxRepository) MarkSent(
	ctx context.Context,
	id int64,
) error {

	const query = `
		UPDATE notification_outbox 
		SET 
			sent_at = now(), 
			last_error = NULL 
		WHERE id = $1
	`
	// Isi sent_at dengan waktu sekarang dan kosongkan last_error.
	_, err := s.db.ExecContext(
		ctx,
		query,
		id,
	)

	return err
}

// MarkRetry menyimpan error terakhir dan menjadwalkan percobaan berikutnya setelah delay.
func (s *OutboxRepository) MarkRetry(
	ctx context.Context,
	id int64,
	cause string,
	delay time.Duration,
) error {

	const query = `
		UPDATE notification_outbox 
		SET 
			last_error = $2, 
			next_attempt_at = now() + make_interval(secs => $3) 
		WHERE id = $1
	`
	// Simpan penyebab (dipotong ke 500 byte) dan geser jadwal percobaan sebesar delay.
	_, err := s.db.ExecContext(
		ctx,
		query,
		id,
		truncate(cause),
		delay.Seconds(),
	)

	return err
}

// MarkFailed menandai baris gagal permanen dan menyimpan penyebabnya (dipotong ke 500 byte).
func (s *OutboxRepository) MarkFailed(
	ctx context.Context,
	id int64,
	cause string,
) error {

	const query = `
		UPDATE notification_outbox 
		SET 
			failed_at = now(), 
			last_error = $2 
		WHERE id = $1
	`
	// Isi failed_at dengan waktu sekarang dan simpan penyebab.
	_, err := s.db.ExecContext(
		ctx,
		query,
		id,
		truncate(cause),
	)

	return err
}

// truncate memotong teks ke 500 byte agar kolom last_error tidak membengkak. error SMTP bisa panjang dan
// tidak ada gunanya menyimpan seluruhnya.
func truncate(s string) string {
	// Teks lebih dari 500 byte: potong. Bisa memotong di tengah karakter UTF-8, tetapi last_error hanya untuk dibaca operator.
	if len(s) > 500 {
		return s[:500]
	}
	// Teks sudah cukup pendek.
	return s
}

// pendingSQL: hitung baris outbox yang belum terkirim dan belum gagal permanen (termasuk yang menunggu percobaan ulang).
const pendingSQL = `SELECT count(*) FROM notification_outbox WHERE sent_at IS NULL AND failed_at IS NULL`

// Pending mengembalikan jumlah baris outbox yang masih menunggu dikirim. Dipakai metrik mail_outbox_pending.
func (s *OutboxRepository) Pending(
	ctx context.Context,
) (int64, error) {
	// satu angka hasil count(*)
	var n int64
	// jalankan query hitung; galat dikembalikan ke pemanggil (metrik melewati sampel)
	if err := s.db.GetContext(ctx, &n, pendingSQL); err != nil {
		return 0, fmt.Errorf("count pending outbox: %w", err)
	}
	return n, nil
}
