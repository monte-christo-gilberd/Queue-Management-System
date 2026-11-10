package repository

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// PushRepository menyimpan langganan push notifikasi di tabel push_subscriptions.
type PushRepository struct {
	// db adalah koneksi PostgreSQL yang dipakai untuk menulis dan menghapus langganan push.
	db *sqlx.DB
}

// NewPushRepository membuat PushRepository baru dengan koneksi database yang diberikan.
func NewPushRepository(db *sqlx.DB) *PushRepository {
	return &PushRepository{db: db}
}

// Upsert menyimpan langganan; endpoint yang sama dipindah ke user yang sedang login (satu browser, satu akun aktif).
func (r *PushRepository) Upsert(ctx context.Context, userID int64, endpoint, p256dh, auth string) error {
	// Jalankan INSERT langganan push; jika endpoint sudah ada, ON CONFLICT memindahkannya ke user ini dan memperbarui kunci p256dh dan auth.
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (endpoint) DO UPDATE
		 SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth`,
		userID, endpoint, p256dh, auth,
	)
	return err
}

// Delete hanya menghapus langganan milik user itu.
func (r *PushRepository) Delete(ctx context.Context, userID int64, endpoint string) error {
	// Jalankan DELETE untuk menghapus langganan dengan endpoint ini, tetapi hanya jika langganan itu milik user ini.
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2`,
		userID, endpoint,
	)
	return err
}
