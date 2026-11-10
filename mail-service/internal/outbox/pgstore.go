package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PG: Store di atas Postgres, memakai role mail_rw (baca outbox, ubah status kirim, baca email pengguna).
type PG struct {
	// pool: pool koneksi ke PostgreSQL.
	pool *pgxpool.Pool
}

// NewPG membuat PG dari pool koneksi yang sudah dibuka.
func NewPG(pool *pgxpool.Pool) *PG { return &PG{pool: pool} }

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
func (s *PG) Claim(ctx context.Context, limit int, lease time.Duration) ([]Item, error) {
	// Jalankan query klaim dengan batas jumlah baris dan lama sewa dalam detik.
	rows, err := s.pool.Query(ctx, claimSQL, limit, lease.Seconds())
	// Query gagal: bungkus galatnya.
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	// Tutup hasil query setelah fungsi selesai.
	defer rows.Close()
	// Kumpulkan baris yang diklaim.
	var out []Item
	for rows.Next() {
		// Siapkan item untuk baris ini.
		var it Item
		// Salin kolom ke item. Galat pemindaian dibungkus dan menghentikan pembacaan.
		if err := rows.Scan(&it.ID, &it.Kind, &it.Email, &it.Attempts, &it.Payload); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		// Tambahkan item ke hasil.
		out = append(out, it)
	}
	// Kembalikan hasil beserta galat yang terjadi saat membaca baris.
	return out, rows.Err()
}

// MarkSent menandai baris terkirim, dan menghapus galat lama.
func (s *PG) MarkSent(ctx context.Context, id int64) error {
	// Isi sent_at dengan waktu sekarang dan kosongkan last_error.
	_, err := s.pool.Exec(ctx, `UPDATE notification_outbox SET sent_at = now(), last_error = NULL WHERE id = $1`, id)
	return err
}

// MarkRetry menyimpan galat terakhir dan menjadwalkan percobaan berikutnya setelah delay.
func (s *PG) MarkRetry(ctx context.Context, id int64, cause string, delay time.Duration) error {
	// Simpan penyebab (dipotong ke 500 karakter) dan geser jadwal percobaan sebesar delay.
	_, err := s.pool.Exec(ctx,
		`UPDATE notification_outbox SET last_error = $2, next_attempt_at = now() + make_interval(secs => $3) WHERE id = $1`,
		id, truncate(cause), delay.Seconds())
	return err
}

// MarkFailed menandai baris gagal permanen dan menyimpan penyebabnya (dipotong ke 500 karakter).
func (s *PG) MarkFailed(ctx context.Context, id int64, cause string) error {
	// Isi failed_at dengan waktu sekarang dan simpan penyebab.
	_, err := s.pool.Exec(ctx, `UPDATE notification_outbox SET failed_at = now(), last_error = $2 WHERE id = $1`, id, truncate(cause))
	return err
}

// truncate memotong teks ke 500 byte agar kolom last_error tidak membengkak.
func truncate(s string) string {
	// Teks lebih dari 500 byte: potong.
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

// Listen memanggil wake setiap NOTIFY mail_outbox, dan menyambung ulang bila koneksi putus. Hanya
// pembangun: bila gagal, worker tetap jalan lewat polling.
func Listen(ctx context.Context, pool *pgxpool.Pool, wake func(), log *slog.Logger) {
	// Ulangi selama konteks belum batal.
	for ctx.Err() == nil {
		// Jalankan satu sesi LISTEN. Galat (dan konteks belum batal) dicatat, lalu disambung ulang.
		if err := listenOnce(ctx, pool, wake); err != nil && ctx.Err() == nil {
			log.Warn("outbox listen lost, reconnecting (polling continues)", "err", err)
		}
		// Tunggu 3 detik sebelum menyambung ulang, kecuali shutdown dimulai.
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

// listenOnce menjalankan satu sesi LISTEN sampai koneksi putus atau konteks batal.
func listenOnce(ctx context.Context, pool *pgxpool.Pool, wake func()) error {
	// Ambil satu koneksi dari pool, khusus untuk mendengarkan notifikasi.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// Kembalikan koneksi ke pool saat sesi selesai.
	defer conn.Release()
	// Daftarkan koneksi ke channel mail_outbox.
	if _, err := conn.Exec(ctx, "LISTEN mail_outbox"); err != nil {
		return err
	}
	// Bangunkan worker sekali setelah tersambung, untuk baris yang menunggu selama koneksi putus.
	wake()
	// Tunggu notifikasi berulang kali.
	for {
		// Tunggu satu notifikasi. Galat (koneksi putus atau konteks batal) menghentikan sesi.
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			// Galat khusus yang tidak seharusnya terjadi di sini: lewati dan tunggu notifikasi berikutnya.
			if err == pgx.ErrNoRows {
				continue
			}
			return err
		}
		// Ada notifikasi: bangunkan worker.
		wake()
	}
}
