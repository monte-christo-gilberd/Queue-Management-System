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

func NewDisplayRepository(db *sqlx.DB) *DisplayRepository {
	return &DisplayRepository{db: db}
}

const displayCols = `queue_id, token, upcoming_count, skipped_count, created_at, rotated_at`

func (r *DisplayRepository) Get(ctx context.Context, queueID int64) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	err := r.db.GetContext(ctx, &d, `SELECT `+displayCols+` FROM queue_displays WHERE queue_id = $1`, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("get display: %w", err)
	}
	return d, nil
}

func (r *DisplayRepository) GetByToken(ctx context.Context, token string) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	err := r.db.GetContext(ctx, &d, `SELECT `+displayCols+` FROM queue_displays WHERE token = $1`, token)
	if errors.Is(err, sql.ErrNoRows) {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("get display by token: %w", err)
	}
	return d, nil
}

// Upsert membuat layar (dengan token yang diberikan) atau, bila sudah ada, hanya mengubah jumlahnya:
// token yang sudah ada tidak diganti.
func (r *DisplayRepository) Upsert(ctx context.Context, queueID int64, token string, upcoming, skipped int) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	err := r.db.GetContext(ctx, &d, `
		INSERT INTO queue_displays (queue_id, token, upcoming_count, skipped_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (queue_id) DO UPDATE
			SET upcoming_count = EXCLUDED.upcoming_count, skipped_count = EXCLUDED.skipped_count
		RETURNING `+displayCols, queueID, token, upcoming, skipped)
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("upsert display: %w", err)
	}
	return d, nil
}

// Rotate mengganti token. apperror.ErrNotFound bila layar belum diaktifkan.
func (r *DisplayRepository) Rotate(ctx context.Context, queueID int64, token string) (model.QueueDisplay, error) {
	var d model.QueueDisplay
	err := r.db.GetContext(ctx, &d,
		`UPDATE queue_displays SET token = $2, rotated_at = now() WHERE queue_id = $1 RETURNING `+displayCols, queueID, token)
	if errors.Is(err, sql.ErrNoRows) {
		return model.QueueDisplay{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.QueueDisplay{}, fmt.Errorf("rotate display: %w", err)
	}
	return d, nil
}

// Delete mematikan layar. Idempoten.
func (r *DisplayRepository) Delete(ctx context.Context, queueID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM queue_displays WHERE queue_id = $1`, queueID); err != nil {
		return fmt.Errorf("delete display: %w", err)
	}
	return nil
}

func (r *DisplayRepository) QueueInfo(ctx context.Context, queueID int64) (model.DisplayQueueInfo, error) {
	var q model.DisplayQueueInfo
	err := r.db.GetContext(ctx, &q, `SELECT queue_name, queue_desc, ticket_prefix, status FROM queues WHERE id = $1`, queueID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.DisplayQueueInfo{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.DisplayQueueInfo{}, fmt.Errorf("display queue info: %w", err)
	}
	return q, nil
}

// UpcomingNumbers: nomor `waiting` terdepan, urut antrian.
func (r *DisplayRepository) UpcomingNumbers(ctx context.Context, queueID int64, limit int) ([]int64, error) {
	return r.numbers(ctx, `
		SELECT ticket_number FROM tickets WHERE queue_id = $1 AND status = 'waiting'
		ORDER BY sort_key ASC LIMIT $2`, queueID, limit)
}

// SkippedNumbers: nomor `skipped` yang paling baru dilewati.
func (r *DisplayRepository) SkippedNumbers(ctx context.Context, queueID int64, limit int) ([]int64, error) {
	return r.numbers(ctx, `
		SELECT ticket_number FROM tickets WHERE queue_id = $1 AND status = 'skipped'
		ORDER BY called_at DESC NULLS LAST, id DESC LIMIT $2`, queueID, limit)
}

func (r *DisplayRepository) numbers(ctx context.Context, query string, queueID int64, limit int) ([]int64, error) {
	out := []int64{}
	if limit <= 0 {
		return out, nil
	}
	if err := r.db.SelectContext(ctx, &out, query, queueID, limit); err != nil {
		return nil, fmt.Errorf("display numbers: %w", err)
	}
	return out, nil
}
