package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore: akses database lewat role payment_rw (db/migrations/05_payment_role.sh). Role itu hanya boleh
// membaca/mengubah owners.subscription_expires_at, menambah baris payments, dan tabel milik payment-service
// sendiri; tidak ada akses ke antrian, tiket, atau data lain.
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PGStore) OwnerEmail(ctx context.Context, ownerID int64) (string, error) {
	var email string
	err := s.pool.QueryRow(ctx,
		`SELECT u.email FROM owners o JOIN users u ON u.id = o.user_id WHERE o.user_id = $1`, ownerID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotOwner
	}
	return email, err
}

func (s *PGStore) RecordEvent(ctx context.Context, eventID, eventType string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO payment_webhook_events (event_id, type) VALUES ($1, $2) ON CONFLICT DO NOTHING`, eventID, eventType)
	return err
}

// Credit: satu transaksi. Baris owners terkunci oleh UPDATE, jadi dua webhook untuk owner yang sama
// diproses berurutan. Gagal di langkah mana pun membatalkan semuanya (event tidak tercatat), sehingga
// Stripe mengirim ulang dan perpanjangan terkejar.
func (s *PGStore) Credit(ctx context.Context, c Credit) (bool, time.Time, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, time.Time{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Event yang sama (Stripe mengirim ulang): abaikan.
	tag, err := tx.Exec(ctx,
		`INSERT INTO payment_webhook_events (event_id, type) VALUES ($1, $2) ON CONFLICT DO NOTHING`, c.EventID, c.EventType)
	if err != nil {
		return false, time.Time{}, err
	}
	if tag.RowsAffected() == 0 {
		return false, time.Time{}, nil
	}

	// 2. Sesi yang sama dari event berbeda (completed lalu async_payment_succeeded): perpanjang sekali saja.
	tag, err = tx.Exec(ctx,
		`INSERT INTO payment_session_credits (session_id) VALUES ($1) ON CONFLICT DO NOTHING`, c.SessionID)
	if err != nil {
		return false, time.Time{}, err
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return false, time.Time{}, err
		}
		return false, time.Time{}, nil
	}

	// 3. Perpanjang tepat 1 bulan: dari tanggal berakhir bila masih aktif, dari sekarang bila sudah lewat.
	var expires time.Time
	err = tx.QueryRow(ctx, `
		UPDATE owners
		   SET subscription_expires_at = GREATEST(now(), subscription_expires_at) + interval '1 month'
		 WHERE user_id = $1
		RETURNING subscription_expires_at`, c.OwnerID).Scan(&expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, time.Time{}, ErrNotOwner
	}
	if err != nil {
		return false, time.Time{}, err
	}

	// 4. Riwayat pembayaran (tabel payments dari skema QMS).
	var paymentID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO payments (owner_id, amount, currency, plan_type, extends_subscription_to, status)
		VALUES ($1, $2::numeric, $3, 'monthly', $4, 'success')
		RETURNING id`, c.OwnerID, c.Amount, c.Currency, expires).Scan(&paymentID)
	if err != nil {
		return false, time.Time{}, err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE payment_session_credits SET payment_id = $2 WHERE session_id = $1`, c.SessionID, paymentID); err != nil {
		return false, time.Time{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, time.Time{}, err
	}
	return true, expires, nil
}
