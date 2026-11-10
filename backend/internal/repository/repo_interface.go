package repository

import (
	"context"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

// QueueRepositoryInterface menggambarkan method yang disediakan QueueRepository (queue_repo.go): antrian, pengaturan skip, QR, dan counter default.
type QueueRepositoryInterface interface {
	CreateNewQueue(
		ctx context.Context,
		tx *sqlx.Tx,
		ownerID int64,
		queueName string,
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
	SetTicketPrefix(
		ctx context.Context,
		queueID int64,
		prefix string,
	) error
	UpdateQueueDetails(
		ctx context.Context,
		queueID int64,
		queueName string,
		queueDescription string,
	) error
	CloseQueue(
		ctx context.Context,
		queueID int64,
	) error
	GetQueueSettings(
		ctx context.Context,
		queueID int64,
	) (model.QueueSettings, error)
	CreateQueueSettingsIfMissing(
		ctx context.Context,
		queueID int64,
	) error
	GetSkipSettings(
		ctx context.Context,
		queueID int64,
	) (model.SkipSettings, error)
	UpdateSkipSettings(
		ctx context.Context,
		queueID int64,
		settings model.SkipSettings,
	) error
	UpdateQRType(
		ctx context.Context,
		queueID int64,
		qrType string,
	) error
	UpdateQRCurrentCode(
		ctx context.Context,
		queueID int64,
		token string,
	) error
	CreateDefaultCounter(
		ctx context.Context,
		queueID int64,
		name string,
	) (model.Counter, error)
	GetCounterByQueue(
		ctx context.Context,
		queueID int64,
	) (model.Counter, error)
}

// SubscriptionCheckerInterface menggambarkan method yang disediakan SubscriptionChecker (owner_repo.go): cek langganan owner aktif.
type SubscriptionCheckerInterface interface {
	IsActive(
		ctx context.Context,
		ownerID int64,
	) (bool, error)
}

// TicketRepositoryInterface menggambarkan method yang disediakan TicketRepository (ticket_repo.go): tiket, pengurutan sort_key, dan perubahan status tiket.
type TicketRepositoryInterface interface {
	MaxWaitingSortKey(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
	) (int64, error)
	Insert(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID int64,
		userID int64,
		sortKey int64,
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
	MarkServing(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
		counterID int64,
	) error
	MarkCancelled(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	MarkDeleted(
		ctx context.Context,
		tx *sqlx.Tx,
		ticketID int64,
	) error
	GetForUpdate(
		ctx context.Context,
		tx *sqlx.Tx,
		queueID,
		ticketID int64,
	) (model.Ticket, error)
}

// UserRepositoryInterface menggambarkan method yang disediakan UserRepository (user_repo.go): pembuatan akun dan pencarian user berdasarkan email.
type UserRepositoryInterface interface {
	Create(
		ctx context.Context,
		email string,
		username string,
		passwordHash string,
	) (model.User, error)
	FindByEmail(
		ctx context.Context,
		email string,
	) (model.User, error)
}
