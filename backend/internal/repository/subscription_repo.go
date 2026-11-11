package repository

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// SubscriptionRepository menambah masa langganan owner sebesar 30 hari secara langsung (tanpa pembayaran).
// Dipenuhi service.SubscriptionService.
type SubscriptionRepository struct {
	// db adalah koneksi PostgreSQL yang dipakai untuk menulis owners dan payments.
	db *sqlx.DB
}

// NewSubscriptionRepository membuat SubscriptionRepository dengan koneksi database yang diberikan.
func NewSubscriptionRepository(
	db *sqlx.DB,
) *SubscriptionRepository {
	return &SubscriptionRepository{
		db: db,
	}
}

// Extend menambah 30 hari ke langganan userID dan mengembalikan tanggal berakhir yang baru.
// Bila user belum punya baris owners, baris dibuat dengan now() + 30 hari. Bila sudah ada dan masih aktif, tambahan
// dihitung dari tanggal berakhir; bila sudah lewat, dihitung dari sekarang. Satu transaksi: baris owners dan riwayat
// payments (amount 0, status success) ditulis bersamaan, jadi keduanya tidak pernah tidak sinkron.
func (r *SubscriptionRepository) Extend(
	ctx context.Context,
	userID int64,
) (time.Time, error) {
	// mulai transaksi; bila gagal, kembalikan error ke pemanggil
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return time.Time{}, err
	}
	// rollback otomatis bila fungsi keluar sebelum commit (no-op setelah commit)
	defer func() { _ = tx.Rollback() }()

	// upsert owners dalam satu statement: baris baru atau perpanjangan atomik, tanpa race antar request
	var expiresAt time.Time
	const upsert = `
		INSERT INTO owners (user_id, subscription_expires_at)
		VALUES ($1, now() + interval '30 days')
		ON CONFLICT (user_id) DO UPDATE
		SET subscription_expires_at = GREATEST(now(), owners.subscription_expires_at) + interval '30 days'
		RETURNING subscription_expires_at
	`
	if err := tx.GetContext(ctx, &expiresAt, upsert, userID); err != nil {
		return time.Time{}, err
	}

	// catat riwayat: penambahan gratis, jadi amount 0 dan plan monthly (satu-satunya plan yang tersisa)
	const history = `
		INSERT INTO payments (owner_id, amount, currency, plan_type, extends_subscription_to, status)
		VALUES ($1, 0, 'IDR', 'monthly', $2, 'success')
	`
	if _, err := tx.ExecContext(ctx, history, userID, expiresAt); err != nil {
		return time.Time{}, err
	}

	// semua langkah berhasil: simpan
	if err := tx.Commit(); err != nil {
		return time.Time{}, err
	}
	return expiresAt, nil
}
