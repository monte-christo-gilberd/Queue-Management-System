package repository

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type TicketRepository struct {
	db *sqlx.DB
}

func NewTicketRepository(
	db *sqlx.DB,
) *TicketRepository {
	return &TicketRepository{
		db: db,
	}
}

func (r *TicketRepository) MaxWaitingSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (int64, error) {
	var max sql.NullInt64

	const query = `
		SELECT MAX(sort_key) 
		FROM tickets 
		WHERE queue_id=$1 AND status='waiting'
	`

	err := tx.GetContext(
		ctx,
		&max,
		query,
		queueID,
	)
	if err != nil {
		return 0, err
	}

	if !max.Valid {
		return 0, nil
	}
	return max.Int64, nil
}

func (r *TicketRepository) Insert(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID, userID, sortKey int64,
) (model.Ticket, error) {

	var newTicket model.Ticket
	const query = `
		INSERT INTO tickets(
			queue_id, 
			ticket_number, 
			sort_key, 
			user_id, 
			status
		)
		VALUES (
			$1, (
			SELECT 
				COALESCE(MAX(ticket_number), 0) + 1 
			FROM tickets 
			WHERE queue_id = $1
			),
		 	$2, 
			$3, 
			'waiting'
		)
		RETURNING 
			id, 
			queue_id, 
			ticket_number, 
			sort_key, 
			user_id, 
			status, 
			skip_count, 
			last_notified_at, 
			created_at, 
			completed_at, 
			cancelled_at
	`

	err := tx.GetContext(
		ctx,
		&newTicket,
		query,
		queueID,
		sortKey,
		userID,
	)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.Ticket{}, apperror.ErrDuplicate
	}
	if err != nil {
		return model.Ticket{}, err
	}
	return newTicket, nil
}

func (r *TicketRepository) NextWaiting(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.Ticket, error) {

	var currentTicket model.Ticket
	const query = `
		SELECT  
			id, 
			queue_id, 
			ticket_number, 
			sort_key, 
			user_id, 
			status, 
			skip_count, 
			last_notified_at, 
			created_at, 
			completed_at, 
			cancelled_at
		FROM tickets 
		WHERE queue_id=$1 AND status='waiting'
		ORDER BY sort_key ASC 
		LIMIT 1 
		FOR UPDATE
	`

	err := tx.GetContext(
		ctx,
		&currentTicket,
		query,
		queueID,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.Ticket{}, err
	}

	return currentTicket, nil
}

func (r *TicketRepository) MarkCompleted(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {

	const query = `
		UPDATE tickets 
		SET 
			status='completed', 
			completed_at=now() 
		WHERE id=$1
	`

	result, err := tx.ExecContext(
		ctx,
		query,
		ticketID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *TicketRepository) ListByQueue(
	ctx context.Context,
	queueID int64,
	statuses []string,
) ([]model.Ticket, error) {

	tickets := []model.Ticket{}

	const selectTickets = `
		SELECT
			id,
			queue_id,
			ticket_number,
			sort_key,
			user_id,
			status,
			skip_count,
			last_notified_at,
			created_at,
			completed_at,
			cancelled_at
		FROM tickets
		WHERE queue_id = ?
	`
	const orderAndLimit = `
		ORDER BY 
			sort_key ASC, 
			id ASC
		LIMIT 500
	`

	// Tanpa filter status: query statis biasa.
	if len(statuses) == 0 {
		err := r.db.SelectContext(
			ctx,
			&tickets,
			r.db.Rebind(selectTickets+orderAndLimit),
			queueID,
		)
		if err != nil {
			return nil, err
		}
		return tickets, nil
	}

	// sqlx.In memperluas slice menjadi daftar placeholder; Rebind mengubah ? menjadi $1, $2, ...
	query, args, err := sqlx.In(
		selectTickets+` AND status IN (?)`+orderAndLimit,
		queueID,
		statuses,
	)
	if err != nil {
		return nil, err
	}

	err = r.db.SelectContext(
		ctx,
		&tickets,
		r.db.Rebind(query),
		args...,
	)
	if err != nil {
		return nil, err
	}

	return tickets, nil
}

// HasTicket melaporkan apakah user pernah punya tiket (status apa pun) di queue ini.
func (r *TicketRepository) HasTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (bool, error) {

	var exists bool

	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM tickets
			WHERE queue_id = $1 AND user_id = $2
		)
	`

	err := r.db.GetContext(
		ctx,
		&exists,
		query,
		queueID,
		userID,
	)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// FindByUser mengembalikan tiket user di queue ini (UNIQUE queue_id+user_id, jadi paling banyak satu).
func (r *TicketRepository) FindByUser(
	ctx context.Context,
	queueID,
	userID int64,
) (model.Ticket, error) {

	var ticket model.Ticket

	const query = `
		SELECT
			id,
			queue_id,
			ticket_number,
			sort_key,
			user_id,
			status,
			skip_count,
			last_notified_at,
			created_at,
			completed_at,
			cancelled_at
		FROM tickets
		WHERE queue_id = $1 AND user_id = $2
	`

	err := r.db.GetContext(
		ctx,
		&ticket,
		query,
		queueID,
		userID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Ticket{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.Ticket{}, err
	}

	return ticket, nil
}

// LastCompletedNumber: ticket_number tiket completed terbaru; ok=false bila belum ada.
func (r *TicketRepository) LastCompletedNumber(
	ctx context.Context,
	queueID int64,
) (number int, ok bool, err error) {

	const query = `
		SELECT ticket_number
		FROM tickets
		WHERE queue_id = $1 AND status = 'completed'
		ORDER BY completed_at DESC, id DESC
		LIMIT 1
	`

	err = r.db.GetContext(
		ctx,
		&number,
		query,
		queueID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	return number, true, nil
}

// CountWaitingBefore: jumlah tiket waiting di queue ini dengan sort_key lebih kecil.
func (r *TicketRepository) CountWaitingBefore(
	ctx context.Context,
	queueID,
	sortKey int64,
) (int, error) {

	var count int

	const query = `
		SELECT COUNT(*)
		FROM tickets
		WHERE queue_id = $1
			AND status = 'waiting'
			AND sort_key < $2
	`

	err := r.db.GetContext(
		ctx,
		&count,
		query,
		queueID,
		sortKey,
	)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// ListByUser mengembalikan tiket milik user di semua antrian, terbaru dulu (maks 100).
func (r *TicketRepository) ListByUser(
	ctx context.Context,
	userID int64,
) ([]model.UserTicket, error) {

	tickets := []model.UserTicket{}

	const query = `
		SELECT
			t.id,
			t.queue_id,
			t.ticket_number,
			t.sort_key,
			t.user_id,
			t.status,
			t.skip_count,
			t.last_notified_at,
			t.created_at,
			t.completed_at,
			t.cancelled_at,
			q.queue_name,
			q.status AS queue_status
		FROM tickets t
		JOIN queues q ON q.id = t.queue_id
		WHERE t.user_id = $1
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT 100
	`

	err := r.db.SelectContext(
		ctx,
		&tickets,
		query,
		userID,
	)
	if err != nil {
		return nil, err
	}

	return tickets, nil
}
