package repository

import (
	"context"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

type QueueRepositoryInterface interface {
	CreateNewQueue(
		ctx context.Context,
		tx *sqlx.Tx,
		ownerID int64,
		queueName,
		queueDescription string,
	) (model.Queue, error)
	GetQueueByID(
		ctx context.Context,
		queueID int64,
	) (model.Queue, error)
	ListQueueByOwner(
		ctx context.Context,
		ownerID int64,
		activeOnly bool,
	) ([]model.Queue, error)
	UpdateQueueDetails(
		ctx context.Context,
		queueID int64,
		queueName,
		queueDescription string,
	) error
	CloseQueue(
		ctx context.Context,
		queueID int64,
	) error
}

type SubscriptionCheckerInterface interface {
	IsActive(
		ctx context.Context,
		ownerID int64,
	) (bool, error)
}

type TicketRepositoryInterface interface {
	MaxWaitingSortKey(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) (int64, error)
	Insert(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID, userID, sortKey int64,
	) (model.Ticket, error)
	NextWaiting(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) (model.Ticket, error)
	MarkCompleted(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
}

type UserRepositoryInterface interface {
	Create(
		ctx context.Context,
		email,
		username,
		passwordHash string,
	) (model.User, error)
	FindByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
}
