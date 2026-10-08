package service

import (
	"context"
	"errors"
	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

type TicketRepository interface {
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

type TicketService struct {
	repository TicketRepository
	tx         TxRunner
	notifier   EventNotifier
}

func NewTicketService(
	repository TicketRepository,
	tx TxRunner,
	notifiers ...EventNotifier,
) *TicketService {
	svc := &TicketService{
		repository: repository,
		tx:         tx,
	}
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

const SortKeyGap int64 = 65536

const maxRetries int = 3

func (s *TicketService) JoinQueue(
	ctx context.Context,
	queueID, userID int64,
) (model.Ticket, error) {
	var retry int = 0
	var sortKey int64
	var ticket model.Ticket

	for retry < maxRetries {
		if err := s.tx.WithTx(
			ctx,
			func(tx *sqlx.Tx) error {
				var err error
				sortKey, err = s.repository.MaxWaitingSortKey(
					ctx,
					tx,
					queueID,
				)
				if err != nil {
					return err
				}

				ticket, err = s.repository.Insert(
					ctx,
					tx,
					queueID,
					userID,
					sortKey+SortKeyGap,
				)
				if err != nil {
					return err
				}

				return nil
			},
		); err != nil {
			if errors.Is(err, apperror.ErrDuplicate) {
				retry++
				continue
			} else {
				return model.Ticket{}, err
			}
		}
		break
	}

	if retry == maxRetries {
		return model.Ticket{}, apperror.ErrRetryExhausted
	}

	notifyEvent(s.notifier, queueID, EventTicketJoined, ticket)

	return ticket, nil
}

func (s *TicketService) NextTicket(
	ctx context.Context,
	queueID int64,
) (model.Ticket, error) {
	var ticket model.Ticket

	err := s.tx.WithTx(
		ctx,
		func(tx *sqlx.Tx) error {
			var err error
			ticket, err = s.repository.NextWaiting(
				ctx,
				tx,
				queueID,
			)
			if err != nil {
				return err
			}
			return s.repository.MarkCompleted(
				ctx,
				tx,
				ticket.ID,
			)
		})

	if errors.Is(err, apperror.ErrNotFound) {
		return model.Ticket{}, apperror.ErrQueueEmpty
	}
	if err != nil {
		return model.Ticket{}, err
	}

	notifyEvent(s.notifier, queueID, EventTicketNext, ticket)

	return ticket, nil
}
