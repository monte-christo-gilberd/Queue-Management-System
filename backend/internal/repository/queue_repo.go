package repository

import (
	"context"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"
	"strings"

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
) (model.Queue, model.QueueAuthz, error) {

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
		return model.Queue{}, model.QueueAuthz{}, err
	}

	var queueAuthz model.QueueAuthz
	const authzQuery = `
		INSERT INTO queue_authz(queue_id)
		VALUES ($1)
		RETURNING 
			queue_id,
			can_manage_queue,
			can_manage_counters,
			can_manage_qr,
			can_view_queue,
			can_manage_display,
			can_continue_queue,
			can_notify_customer,
			can_manage_tickets,
			can_manage_notes
	`

	err = tx.GetContext(
		ctx,
		&queueAuthz,
		authzQuery,
		queue.ID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	return queue, queueAuthz, nil
}

func (r *QueueRepository) GetQueueByID(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.Queue, model.QueueAuthz, error) {

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

	err := tx.GetContext(
		ctx,
		&queue,
		queueQuery,
		queueID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	var queueAuthz model.QueueAuthz
	const authzQuery = `
		SELECT
			queue_id,
			can_manage_queue,
			can_manage_counters,
			can_manage_qr,
			can_view_queue,
			can_manage_display,
			can_continue_queue,
			can_notify_customer,
			can_manage_tickets,
			can_manage_notes
		FROM queue_authz
		WHERE queue_id = $1
	`

	err = tx.GetContext(
		ctx,
		&queueAuthz,
		authzQuery,
		queue.ID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	return queue, queueAuthz, nil
}

func (r *QueueRepository) ListQueueByOwner(
	ctx context.Context,
	tx *sqlx.Tx,
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
	err := tx.SelectContext(
		ctx,
		&queues,
		query,
		ownerID,
	)
	if err != nil {
		return []model.Queue{}, err
	}

	return queues, nil
}

func (r *QueueRepository) UpdateQueueDetails(
	ctx context.Context,
	tx *sqlx.Tx,
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

	result, err := tx.ExecContext(
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

func (r *QueueRepository) UpdateQueueAuthz(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	authz model.QueueAuthz,
) error {

	var queueAuthz model.QueueAuthz
	const authzQuery = `
		SELECT
			queue_id,
			can_manage_queue,
			can_manage_counters,
			can_manage_qr,
			can_view_queue,
			can_manage_display,
			can_continue_queue,
			can_notify_customer,
			can_manage_tickets,
			can_manage_notes
		FROM queue_authz
		WHERE queue_id = $1
	`

	err := tx.GetContext(
		ctx,
		&queueAuthz,
		authzQuery,
		queueID,
	)
	if err != nil {
		return err
	}

	var updates []string

	if authz.CanManageQueue {
		updates = append(updates, "can_manage_queue = NOT can_manage_queue")
	}
	if authz.CanManageCounters {
		updates = append(updates, "can_manage_counters = NOT can_manage_counters")
	}
	if authz.CanManageQr {
		updates = append(updates, "can_manage_qr = NOT can_manage_qr")
	}
	if authz.CanViewQueue {
		updates = append(updates, "can_view_queue = NOT can_view_queue")
	}
	if authz.CanManageDisplay {
		updates = append(updates, "can_manage_display = NOT can_manage_display")
	}
	if authz.CanContinueQueue {
		updates = append(updates, "can_continue_queue = NOT can_continue_queue")
	}
	if authz.CanNotifyCustomer {
		updates = append(updates, "can_notify_customer = NOT can_notify_customer")
	}
	if authz.CanManageTickets {
		updates = append(updates, "can_manage_tickets = NOT can_manage_tickets")
	}
	if authz.CanManageNotes {
		updates = append(updates, "can_manage_notes = NOT can_manage_notes")
	}

	if len(updates) == 0 {
		return nil
	}

	authzInjectQuery := fmt.Sprintf(`
    UPDATE queue_authz
    SET %s
    WHERE queue_id = $1
`, strings.Join(updates, ", "))

	result, err := tx.ExecContext(
		ctx,
		authzInjectQuery,
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
	tx *sqlx.Tx,
	queueID int64,
) error {

	const query = `
		UPDATE queues
		SET status = 'closed'
		WHERE
			id = $1 AND status = 'active'
	`
	result, err := tx.ExecContext(
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
