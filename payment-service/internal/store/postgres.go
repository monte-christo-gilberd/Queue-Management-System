// Package store menyimpan pesanan pembayaran di PostgreSQL dan menerapkan pembayaran lunas ke langganan owner.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"qms/payment-service/internal/order"
)

// subscriptionPeriod: lama perpanjangan per pembayaran paket bulanan. Sama dengan perpanjangan di backend.
const subscriptionPeriod = "30 days"

// Postgres: store pesanan berbasis PostgreSQL.
type Postgres struct {
	db *sqlx.DB
}

// NewPostgres membuat store dari koneksi database yang sudah dibuka.
func NewPostgres(db *sqlx.DB) *Postgres {
	return &Postgres{db: db}
}

// Ping memeriksa koneksi database (dipakai endpoint health).
func (p *Postgres) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// CreatePending menyimpan pesanan baru berstatus pending. Mengembalikan order.ErrDuplicate bila reference_id sudah ada.
func (p *Postgres) CreatePending(ctx context.Context, o order.Pending) error {
	// Pesanan baru selalu mulai pending; pay_url diisi setelah BorderPay membalas.
	res, err := p.db.ExecContext(ctx,
		`INSERT INTO payment_orders (reference_id, owner_id, amount, plan_type, status)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (reference_id) DO NOTHING`,
		o.ReferenceID, o.OwnerID, o.Amount, o.PlanType, order.StatusPending,
	)
	if err != nil {
		return fmt.Errorf("store: simpan pesanan: %w", err)
	}
	// Tidak ada baris yang masuk berarti reference_id sudah ada.
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: baca hasil simpan: %w", err)
	}
	if n == 0 {
		return order.ErrDuplicate
	}
	return nil
}

// SetPayURL menyimpan pay_url dari BorderPay ke pesanan.
func (p *Postgres) SetPayURL(ctx context.Context, referenceID, payURL string) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_orders SET pay_url = $2, updated_at = now() WHERE reference_id = $1`,
		referenceID, payURL,
	)
	if err != nil {
		return fmt.Errorf("store: simpan pay_url: %w", err)
	}
	return nil
}

// MarkPaid menerapkan pembayaran lunas dalam satu transaksi: status pesanan menjadi paid, langganan owner
// diperpanjang, dan satu baris payments dicatat. Pesanan yang sudah paid tidak diterapkan lagi (applied=false).
// Status expired yang lalu menjadi paid tetap diterapkan, karena paid adalah status final.
func (p *Postgres) MarkPaid(ctx context.Context, referenceID string, amount int64) (bool, error) {
	tx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: mulai transaksi: %w", err)
	}
	defer tx.Rollback()

	// Kunci baris pesanan agar dua webhook kembar tidak menerapkan pembayaran dua kali.
	var o struct {
		OwnerID  int64  `db:"owner_id"`
		Amount   int64  `db:"amount"`
		PlanType string `db:"plan_type"`
		Status   string `db:"status"`
	}
	err = tx.GetContext(ctx, &o,
		`SELECT owner_id, amount, plan_type, status FROM payment_orders WHERE reference_id = $1 FOR UPDATE`,
		referenceID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, order.ErrUnknown
	}
	if err != nil {
		return false, fmt.Errorf("store: baca pesanan: %w", err)
	}
	// Nominal harus sama persis dengan pesanan; selain itu pembayaran tidak diterapkan.
	if o.Amount != amount {
		return false, order.ErrAmountMismatch
	}
	// Sudah lunas sebelumnya: pengiriman ulang tidak mengubah apa pun.
	if o.Status == order.StatusPaid {
		return false, tx.Commit()
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE payment_orders SET status = $2, paid_at = now(), updated_at = now() WHERE reference_id = $1`,
		referenceID, order.StatusPaid,
	); err != nil {
		return false, fmt.Errorf("store: tandai lunas: %w", err)
	}

	// Buat baris owners bila belum ada (calon owner pertama kali berlangganan), atau perpanjang yang sudah ada.
	// Perpanjangan dihitung dari sekarang atau dari akhir langganan yang masih berjalan, mana yang lebih besar.
	var expiresAt time.Time
	err = tx.GetContext(ctx, &expiresAt,
		`INSERT INTO owners (user_id, subscription_expires_at)
		 VALUES ($1, now() + $2::interval)
		 ON CONFLICT (user_id) DO UPDATE
		 SET subscription_expires_at = GREATEST(now(), owners.subscription_expires_at) + $2::interval
		 RETURNING subscription_expires_at`,
		o.OwnerID, subscriptionPeriod,
	)
	if err != nil {
		return false, fmt.Errorf("store: perpanjang langganan: %w", err)
	}

	// Catat pembayaran di riwayat langganan.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO payments (owner_id, amount, currency, plan_type, extends_subscription_to, status)
		 VALUES ($1, $2, 'IDR', $3, $4, 'success')`,
		o.OwnerID, o.Amount, o.PlanType, expiresAt,
	); err != nil {
		return false, fmt.Errorf("store: catat pembayaran: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit: %w", err)
	}
	return true, nil
}
