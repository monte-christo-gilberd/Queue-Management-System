package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

type HistoryRepository struct {
	db *sqlx.DB
}

func NewHistoryRepository(db *sqlx.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

// Kolom ringkasan antrian (dipakai daftar dan detail). closed_at jatuh ke created_at bila kosong, supaya
// antrian lama yang belum di-backfill tetap punya urutan.
const historyQueueColumns = `
	q.id,
	q.queue_name,
	q.ticket_prefix,
	q.created_at AS opened_at,
	COALESCE(q.closed_at, q.created_at) AS closed_at,
	COUNT(t.id) AS total,
	COUNT(t.id) FILTER (WHERE t.status = 'completed') AS completed,
	COUNT(t.id) FILTER (WHERE t.status = 'skipped') AS skipped,
	COUNT(t.id) FILTER (WHERE t.status = 'cancelled') AS cancelled,
	COUNT(t.id) FILTER (WHERE t.status = 'deleted') AS deleted,
	COUNT(t.id) FILTER (WHERE t.status IN ('waiting', 'currently_served')) AS unserved
`

// ListClosedQueues mengembalikan antrian milik owner yang sudah ditutup, terbaru dulu (keyset). Antrian aktif
// tidak pernah muncul (D1). `after` nil = halaman pertama; dipakai batas sangat jauh agar satu query melayani
// keduanya tanpa cabang NULL.
func (r *HistoryRepository) ListClosedQueues(
	ctx context.Context,
	ownerID int64,
	after *model.HistoryQueueCursor,
	limit int,
) ([]model.HistoryQueue, error) {

	afterTime := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	afterID := int64(math.MaxInt64)
	if after != nil {
		afterTime, afterID = after.ClosedAt, after.ID
	}

	queues := []model.HistoryQueue{}
	const query = `
		SELECT ` + historyQueueColumns + `
		FROM queues q
		LEFT JOIN tickets t ON t.queue_id = q.id
		WHERE q.owner_id = $1
		  AND q.status = 'closed'
		  AND (COALESCE(q.closed_at, q.created_at), q.id) < ($2, $3)
		GROUP BY q.id
		ORDER BY COALESCE(q.closed_at, q.created_at) DESC, q.id DESC
		LIMIT $4
	`
	if err := r.db.SelectContext(ctx, &queues, query, ownerID, afterTime, afterID, limit); err != nil {
		return nil, err
	}
	return queues, nil
}

// GetClosedQueue mengembalikan ringkasan satu antrian yang sudah ditutup. Bukan milik owner, belum ditutup,
// atau tidak ada: ErrNotFound (sama untuk ketiganya supaya tidak membocorkan keberadaan antrian).
func (r *HistoryRepository) GetClosedQueue(
	ctx context.Context,
	ownerID,
	queueID int64,
) (model.HistoryQueue, error) {

	var queue model.HistoryQueue
	const query = `
		SELECT ` + historyQueueColumns + `
		FROM queues q
		LEFT JOIN tickets t ON t.queue_id = q.id
		WHERE q.id = $1 AND q.owner_id = $2 AND q.status = 'closed'
		GROUP BY q.id
	`
	err := r.db.GetContext(ctx, &queue, query, queueID, ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.HistoryQueue{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.HistoryQueue{}, err
	}
	return queue, nil
}

// ListQueueTickets mengembalikan nomor di satu antrian, urut nomor naik, setelah `afterNumber` (keyset).
// Nama counter hanya diisi bila antrian punya lebih dari satu counter (counter default disembunyikan,
// sama dengan UI). Catatan ikut lewat LEFT JOIN; pemanggil sudah memastikan requester adalah owner.
func (r *HistoryRepository) ListQueueTickets(
	ctx context.Context,
	queueID int64,
	afterNumber,
	limit int,
) ([]model.HistoryTicket, error) {

	tickets := []model.HistoryTicket{}
	const query = `
		WITH multi AS (
			SELECT COUNT(*) > 1 AS several FROM counters WHERE queue_id = $1
		)
		SELECT
			t.id AS ticket_id,
			t.ticket_number,
			t.status,
			t.skip_count,
			t.created_at AS joined_at,
			t.called_at,
			t.completed_at,
			t.cancelled_at,
			u.username AS customer_name,
			u.email AS customer_email,
			CASE WHEN (SELECT several FROM multi) THEN c.name END AS counter_name,
			n.note
		FROM tickets t
		JOIN users u ON u.id = t.user_id
		LEFT JOIN counters c ON c.id = t.called_counter_id
		LEFT JOIN ticket_notes n ON n.ticket_id = t.id
		WHERE t.queue_id = $1 AND t.ticket_number > $2
		ORDER BY t.ticket_number ASC
		LIMIT $3
	`
	if err := r.db.SelectContext(ctx, &tickets, query, queueID, afterNumber, limit); err != nil {
		return nil, err
	}
	return tickets, nil
}
