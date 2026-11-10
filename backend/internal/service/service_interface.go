package service

import (
	"context"
	"qms/internal/model"
)

// QueueServiceInterface: kontrak service antrian yang dipakai handler dan router. Dipenuhi *QueueService.
type QueueServiceInterface interface {
	CreateQueue(
		ctx context.Context,
		ownerID int64,
		queueName,
		queueDescription string,
	) (model.Queue, error)
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, bool, error)
	ListQueue(
		ctx context.Context,
		requesterID int64,
	) ([]model.Queue, error)
	UpdateQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
		queueName,
		queueDescription string,
	) error
	CloseQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) error
}

// TicketServiceInterface: kontrak service tiket yang dipakai handler dan router. Dipenuhi *TicketService.
type TicketServiceInterface interface {
	JoinQueue(
		ctx context.Context,
		queueID,
		userID int64,
	) (model.Ticket, error)
	NextTicket(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Ticket, error)
	ListTickets(
		ctx context.Context,
		queueID,
		requesterID int64,
		statuses []string,
	) ([]model.Ticket, error)
	QueueStatus(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.QueueStatus, error)
}

// UserServiceInterface: kontrak service akun yang dipakai handler dan router. Dipenuhi *UserService.
type UserServiceInterface interface {
	Register(
		ctx context.Context,
		email,
		username,
		password string,
	) (string, error)
	Login(
		ctx context.Context,
		email,
		password string,
	) (string, error)
}

// EventNotifierInterface: kontrak pengirim event untuk handler. Bentuknya sama dengan EventNotifier.
type EventNotifierInterface interface {
	Notify(
		queueID int64,
		action string,
		payload interface{},
	)
}
