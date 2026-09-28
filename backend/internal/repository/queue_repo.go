package repository

import (
	"context"
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
	ownerID int64,
	Name string,
	Description string,
) (model.Queue, error) {

	var queue model.Queue
	const query = `
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

	err := r.db.GetContext(
		ctx,
		&queue,
		query,
		ownerID,
		Name,
		Description,
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
		WHERE id = $1
	`

	err := r.db.GetContext(
		ctx,
		&queue,
		query,
		queueID,
	)
	if err != nil {
		return model.Queue{}, err
	}

	return queue, nil
}

func (r *QueueRepository) ListQueueByOwner(
	ctx context.Context,
	ownerID int64,
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
		ORDER BY created_at DESC
		LIMIT 10
	`
	err := r.db.SelectContext(
		ctx,
		&queues,
		query,
		ownerID,
	)
	if err != nil {
		return []model.Queue{}, nil
	}

	return queues, nil
}
