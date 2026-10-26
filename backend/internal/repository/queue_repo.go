package repository

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

type QueueRepository struct {
	db *sqlx.DB
}

func NewQueueRepository(
	db *sqlx.DB,
) *QueueRepository {
	return &QueueRepository{
		db: db,
	}
}

func (r *QueueRepository) CreateNewQueue(
	ctx context.Context,
	tx *sqlx.Tx,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, error) {

	var queue model.Queue
	const queueQuery = `
		INSERT INTO queues(
			owner_id, 
			queue_name, 
			queue_desc, 
			status, 
			created_at
		)
		VALUES (
			$1,
			$2,
			$3,
			'active',
			now()		
		)
		RETURNING
			id,
			owner_id, 
			queue_name, 
			queue_desc, 
			status, 
			created_at
	`

	err := tx.GetContext(
		ctx,
		&queue,
		queueQuery,
		ownerID,
		queueName,
		queueDescription,
	)
	if err != nil {
		return model.Queue{}, err
	}
	return queue, nil
}

func (r *QueueRepository) GetQueueByID(
	ctx context.Context,
	queueID int64,
) (model.Queue, error) {

	var queue model.Queue
	const queueQuery = `
		SELECT
			id,
			owner_id, 
			queue_name, 
			queue_desc, 
			status, 
			created_at,
			closed_at
		FROM queues
		WHERE id = $1
	`

	err := r.db.GetContext(
		ctx,
		&queue,
		queueQuery,
		queueID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Queue{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.Queue{}, err
	}

	return queue, nil
}

func (r *QueueRepository) ListQueueByOwner(
	ctx context.Context,
	ownerID int64,
	activeOnly bool,
) ([]model.Queue, error) {

	var queues []model.Queue
	const query = `
		SELECT
			id,
			owner_id, 
			queue_name, 
			queue_desc, 
			status, 
			created_at,
			closed_at
		FROM queues
		WHERE owner_id = $1
		AND ($2 = false OR status = 'active')
		ORDER BY created_at DESC
	`
	err := r.db.SelectContext(
		ctx,
		&queues,
		query,
		ownerID,
		activeOnly,
	)
	if err != nil {
		return []model.Queue{}, err
	}

	return queues, nil
}

func (r *QueueRepository) UpdateQueueDetails(
	ctx context.Context,
	queueID int64,
	queueName,
	queueDescription string,
) error {

	const query = `
		UPDATE queues
		SET 
			queue_name = COALESCE(NULLIF($1, ''), queue_name),
			queue_desc = COALESCE(NULLIF($2, ''), queue_desc)
		WHERE id = $3
	`

	result, err := r.db.ExecContext(
		ctx,
		query,
		queueName,
		queueDescription,
		queueID,
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

func (r *QueueRepository) CloseQueue(
	ctx context.Context,
	queueID int64,
) error {

	const query = `
		UPDATE queues
		SET status = 'closed'
		WHERE
			id = $1 AND status = 'active'
	`
	result, err := r.db.ExecContext(
		ctx,
		query,
		queueID,
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
