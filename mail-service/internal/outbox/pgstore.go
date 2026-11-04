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
type PG struct{ pool *pgxpool.Pool }

func NewPG(pool *pgxpool.Pool) *PG { return &PG{pool: pool} }

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

func (s *PG) Claim(ctx context.Context, limit int, lease time.Duration) ([]Item, error) {
	rows, err := s.pool.Query(ctx, claimSQL, limit, lease.Seconds())
	if err != nil {
		return nil, fmt.Errorf("claim outbox: %w", err)
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Kind, &it.Email, &it.Attempts, &it.Payload); err != nil {
			return nil, fmt.Errorf("scan outbox: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *PG) MarkSent(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE notification_outbox SET sent_at = now(), last_error = NULL WHERE id = $1`, id)
	return err
}

func (s *PG) MarkRetry(ctx context.Context, id int64, cause string, delay time.Duration) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notification_outbox SET last_error = $2, next_attempt_at = now() + make_interval(secs => $3) WHERE id = $1`,
		id, truncate(cause), delay.Seconds())
	return err
}

func (s *PG) MarkFailed(ctx context.Context, id int64, cause string) error {
	_, err := s.pool.Exec(ctx, `UPDATE notification_outbox SET failed_at = now(), last_error = $2 WHERE id = $1`, id, truncate(cause))
	return err
}

func truncate(s string) string {
	if len(s) > 500 {
		return s[:500]
	}
	return s
}

// Listen memanggil wake setiap NOTIFY mail_outbox, dan menyambung ulang bila koneksi putus. Hanya
// pembangun: bila gagal, worker tetap jalan lewat polling.
func Listen(ctx context.Context, pool *pgxpool.Pool, wake func(), log *slog.Logger) {
	for ctx.Err() == nil {
		if err := listenOnce(ctx, pool, wake); err != nil && ctx.Err() == nil {
			log.Warn("outbox listen lost, reconnecting (polling continues)", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func listenOnce(ctx context.Context, pool *pgxpool.Pool, wake func()) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN mail_outbox"); err != nil {
		return err
	}
	wake() // baris yang menunggu selama koneksi putus
	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			if err == pgx.ErrNoRows { // tidak terjadi; jaga-jaga
				continue
			}
			return err
		}
		wake()
	}
}
