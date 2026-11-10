package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

// DisplayRepository: konfigurasi layar publik (queue_displays) dan query data layar.
type DisplayRepository struct {
	db *sqlx.DB
}

// NewDisplayRepository membuat DisplayRepository yang membaca dan menulis lewat *sqlx.DB.
func NewDisplayRepository(db *sqlx.DB) *DisplayRepository {
	return &DisplayRepository{db: db}
}

// displayCols: daftar kolom tabel queue_displays yang dipakai di setiap SELECT dan RETURNING, supaya urutannya seragam.
const displayCols = `queue_id, token, upcoming_count, skipped_count, created_at, rotated_at`

// Get mengambil konfigurasi layar untuk satu antrian. Layar belum diaktifkan: ErrNotFound.
func (r *DisplayRepository) Get(ctx context.Context, queueID int64) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	// Query ini mengambil kolom layar (displayCols) milik antrian ini; hasilnya diisi ke d.
	err := r.db.GetContext(ctx, &d, `SELECT `+displayCols+` FROM queue_displays WHERE queue_id = $1`, queueID)
	// Bila tidak ada baris (layar belum diaktifkan), kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	// Error database lain dibungkus dengan konteks 'get display' lalu dikembalikan.
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("get display: %w", err)
	}
	return d, nil
}

// GetByToken mengambil konfigurasi layar berdasarkan token publiknya (dipakai halaman layar tanpa login).
// Token tidak ditemukan: ErrNotFound.
func (r *DisplayRepository) GetByToken(ctx context.Context, token string) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	// Query ini mengambil kolom layar (displayCols) yang tokennya cocok; hasilnya diisi ke d.
	err := r.db.GetContext(ctx, &d, `SELECT `+displayCols+` FROM queue_displays WHERE token = $1`, token)
	// Bila token tidak cocok dengan layar mana pun, kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	// Error database lain dibungkus dengan konteks 'get display by token' lalu dikembalikan.
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("get display by token: %w", err)
	}
	return d, nil
}

// Upsert membuat layar (dengan token yang diberikan) atau, bila sudah ada, hanya mengubah jumlahnya:
// token yang sudah ada tidak diganti.
func (r *DisplayRepository) Upsert(ctx context.Context, queueID int64, token string, upcoming, skipped int) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	// Query ini menyisipkan baris layar baru; jika antrian sudah punya layar (ON CONFLICT), hanya jumlah upcoming dan skipped yang diperbarui, token lama tetap. Hasil baris dikembalikan lewat RETURNING ke d.
	err := r.db.GetContext(ctx, &d, `
		INSERT INTO queue_displays (queue_id, token, upcoming_count, skipped_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (queue_id) DO UPDATE
			SET upcoming_count = EXCLUDED.upcoming_count, skipped_count = EXCLUDED.skipped_count
		RETURNING `+displayCols, queueID, token, upcoming, skipped)
	// Bila query gagal, kembalikan error dengan konteks 'upsert display'.
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("upsert display: %w", err)
	}
	return d, nil
}

// Rotate mengganti token. apperror.ErrNotFound bila layar belum diaktifkan.
func (r *DisplayRepository) Rotate(ctx context.Context, queueID int64, token string) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	// Query ini mengganti token layar milik antrian ini dan mencatat waktu rotasi (rotated_at) sekarang; baris hasilnya diisi ke d.
	err := r.db.GetContext(ctx, &d,
		`UPDATE queue_displays SET token = $2, rotated_at = now() WHERE queue_id = $1 RETURNING `+displayCols, queueID, token)
	// Bila tidak ada baris yang diperbarui (layar belum ada), kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	// Error database lain dibungkus dengan konteks 'rotate display' lalu dikembalikan.
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("rotate display: %w", err)
	}
	return d, nil
}

// Delete mematikan layar. Idempoten.
func (r *DisplayRepository) Delete(ctx context.Context, queueID int64) error {
	// Query ini menghapus baris layar milik antrian ini; jika baris tidak ada, tidak terjadi apa-apa (idempoten).
	if _, err := r.db.ExecContext(ctx, `DELETE FROM queue_displays WHERE queue_id = $1`, queueID); err != nil {
		// Bila query gagal, bungkus error dengan konteks 'delete display' lalu kembalikan.
		return fmt.Errorf("delete display: %w", err)
	}
	return nil
}

// QueueInfo mengambil nama, deskripsi, prefix tiket, dan status antrian yang ditampilkan di layar publik.
// Antrian tidak ada: ErrNotFound.
func (r *DisplayRepository) QueueInfo(ctx context.Context, queueID int64) (model.DisplayQueueInfo, error) {
	var q model.DisplayQueueInfo
	// Query ini mengambil informasi dasar antrian dengan id ini; hasilnya diisi ke q.
	err := r.db.GetContext(ctx, &q, `SELECT queue_name, queue_desc, ticket_prefix, status FROM queues WHERE id = $1`, queueID)
	// Bila antrian tidak ada, kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.DisplayQueueInfo{}, apperror.ErrNotFound
	}
	// Error database lain dibungkus dengan konteks 'display queue info' lalu dikembalikan.
	if err != nil {
		return model.DisplayQueueInfo{}, fmt.Errorf("display queue info: %w", err)
	}
	return q, nil
}

// UpcomingNumbers: nomor `waiting` terdepan, urut antrian.
func (r *DisplayRepository) UpcomingNumbers(ctx context.Context, queueID int64, limit int) ([]int64, error) {
	// Ambil nomor tiket berstatus waiting di antrian ini, diurutkan sort_key, sebanyak limit; hasilnya dikerjakan oleh helper numbers.
	return r.numbers(ctx, `
		SELECT ticket_number FROM tickets WHERE queue_id = $1 AND status = 'waiting'
		ORDER BY sort_key ASC LIMIT $2`, queueID, limit)
}

// SkippedNumbers: nomor `skipped` yang paling baru dilewati.
func (r *DisplayRepository) SkippedNumbers(ctx context.Context, queueID int64, limit int) ([]int64, error) {
	// Ambil nomor tiket berstatus skipped di antrian ini, yang paling baru dipanggil dulu (NULL di akhir), sebanyak limit; hasilnya dikerjakan oleh helper numbers.
	return r.numbers(ctx, `
		SELECT ticket_number FROM tickets WHERE queue_id = $1 AND status = 'skipped'
		ORDER BY called_at DESC NULLS LAST, id DESC LIMIT $2`, queueID, limit)
}

// numbers: helper bersama untuk query daftar nomor. Limit <= 0 langsung mengembalikan slice kosong tanpa query.
func (r *DisplayRepository) numbers(ctx context.Context, query string, queueID int64, limit int) ([]int64, error) {
	out := []int64{}
	// Jika limit tidak positif, tidak ada yang perlu diambil: kembalikan slice kosong tanpa menjalankan query.
	if limit <= 0 {
		return out, nil
	}
	// Jalankan query yang dikirim pemanggil dengan parameter queueID dan limit; hasilnya diisi ke out.
	if err := r.db.SelectContext(ctx, &out, query, queueID, limit); err != nil {
		// Bila query gagal, bungkus error dengan konteks 'display numbers' lalu kembalikan.
		return nil, fmt.Errorf("display numbers: %w", err)
	}
	return out, nil
}
