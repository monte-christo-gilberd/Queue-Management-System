package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// SubscriptionChecker mengecek status langganan owner di tabel owners. Dipenuhi service.SubscriptionChecker (queue_service.go).
type SubscriptionChecker struct {
	// db adalah koneksi PostgreSQL yang dipakai untuk membaca tanggal kedaluwarsa langganan.
	db *sqlx.DB
}

// NewSubscriptionChecker membuat SubscriptionChecker baru dengan koneksi database yang diberikan.
func NewSubscriptionChecker(
	db *sqlx.DB,
) *SubscriptionChecker {
	return &SubscriptionChecker{
		db: db,
	}
}

// IsActive mengecek apakah langganan owner masih berlaku saat ini. Owner tanpa baris langganan dianggap tidak aktif.
func (r *SubscriptionChecker) IsActive(
	ctx context.Context,
	ownerID int64,
) (bool, error) {

	var subscriptionExpiresAt time.Time
	const query = `
		SELECT subscription_expires_at
		FROM owners
		WHERE user_id = $1
	`

	// Jalankan query yang mengambil tanggal kedaluwarsa langganan milik owner ini, lalu isi ke subscriptionExpiresAt.
	err := r.db.GetContext(
		ctx,
		&subscriptionExpiresAt,
		query,
		ownerID,
	)
	// Jika owner tidak punya baris langganan, anggap tidak aktif dan kembalikan false tanpa error.
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	// Jika terjadi error database lain, kembalikan error aslinya ke pemanggil.
	if err != nil {
		return false, err
	}

	// Bandingkan waktu sekarang dengan tanggal kedaluwarsa; jika waktu sekarang masih sebelum tanggal itu, langganan masih aktif.
	if time.Now().Before(subscriptionExpiresAt) {
		return true, nil
	}

	return false, nil
}
