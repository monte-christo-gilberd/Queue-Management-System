package service

import (
	"context"
	"qms/internal/model"
)

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

type EventNotifierInterface interface {
	Notify(
		queueID int64,
		action string,
		payload interface{},
	)
}
