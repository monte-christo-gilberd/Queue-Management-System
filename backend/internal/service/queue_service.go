package service

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

type QueueRepository interface {
	CreateNewQueue(
		ctx context.Context,
		tx *sqlx.Tx,
		ownerID int64,
		queueName,
		queueDescription string,
	) (model.Queue, model.QueueAuthz, error)
	GetQueueByID(
		ctx context.Context,
		queueID int64,
	) (model.Queue, model.QueueAuthz, error)
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
	UpdateQueueAuthz(
		ctx context.Context,
		queueID int64,
		authz model.QueueAuthz,
	) (model.QueueAuthz, error)
	CloseQueue(
		ctx context.Context,
		queueID int64,
	) error
}
type SubscriptionChecker interface {
	IsActive(
		ctx context.Context,
		ownerID int64,
	) (bool, error)
}

// TicketHolderChecker melaporkan apakah user punya tiket di sebuah queue. Dipenuhi TicketRepository.
type TicketHolderChecker interface {
	HasTicket(
		ctx context.Context,
		queueID,
		userID int64,
	) (bool, error)
}

type QueueService struct {
	repository   QueueRepository
	subscription SubscriptionChecker
	tx           TxRunner
	notifier     EventNotifier
	holders      TicketHolderChecker
}

func NewQueueService(
	repository QueueRepository,
	subscription SubscriptionChecker,
	tx TxRunner,
	notifiers ...EventNotifier,
) *QueueService {
	svc := &QueueService{
		repository:   repository,
		subscription: subscription,
		tx:           tx,
	}
	if len(notifiers) > 0 {
		svc.notifier = notifiers[0]
	}
	return svc
}

// WithTicketHolderChecker mengaktifkan akses baca queue untuk pemegang tiket (lihat ViewQueue).
func (s *QueueService) WithTicketHolderChecker(holders TicketHolderChecker) *QueueService {
	s.holders = holders
	return s
}

func (s *QueueService) CreateQueue(
	ctx context.Context,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, model.QueueAuthz, error) {

	err := s.isSubscribed(
		ctx,
		ownerID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	var queue model.Queue
	var queueAuthz model.QueueAuthz

	err = s.tx.WithTx(
		ctx,
		func(tx *sqlx.Tx) error {
			var err error
			queue, queueAuthz, err = s.repository.CreateNewQueue(
				ctx,
				tx,
				ownerID,
				queueName,
				queueDescription,
			)
			if err != nil {
				return err
			}
			return nil
		},
	)

	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}
	return queue, queueAuthz, nil
}

func (s *QueueService) GetQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, model.QueueAuthz, error) {

	queue, queueAuthz, err := s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	return queue, queueAuthz, nil
}

// ViewQueue: owner mendapat queue + authz (isOwner true); pemegang tiket mendapat queue saja
// (authz kosong, isOwner false); selain itu apperror.ErrNotFound.
func (s *QueueService) ViewQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, model.QueueAuthz, bool, error) {

	queue, queueAuthz, err := s.repository.GetQueueByID(
		ctx,
		queueID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, false, err
	}

	if requesterID == queue.OwnerID {
		return queue, queueAuthz, true, nil
	}

	if s.holders != nil {
		isHolder, err := s.holders.HasTicket(
			ctx,
			queueID,
			requesterID,
		)
		if err != nil {
			return model.Queue{}, model.QueueAuthz{}, false, err
		}
		if isHolder {
			return queue, model.QueueAuthz{}, false, nil
		}
	}

	return model.Queue{}, model.QueueAuthz{}, false, apperror.ErrNotFound
}

// GetActiveQueue memastikan queue ada (ErrNotFound) dan masih aktif (ErrQueueClosed).
func (s *QueueService) GetActiveQueue(
	ctx context.Context,
	queueID int64,
) (model.Queue, error) {

	queue, _, err := s.repository.GetQueueByID(
		ctx,
		queueID,
	)
	if err != nil {
		return model.Queue{}, err
	}
	if queue.Status != "active" {
		return model.Queue{}, apperror.ErrQueueClosed
	}

	return queue, nil
}

func (s *QueueService) ListQueue(
	ctx context.Context,
	requesterID int64,
) ([]model.Queue, error) {

	err := s.isSubscribed(
		ctx,
		requesterID,
	)

	activeOnly := true
	if err == nil {
		activeOnly = false
	}

	queues, err := s.repository.ListQueueByOwner(
		ctx,
		requesterID,
		activeOnly,
	)
	if err != nil {
		return []model.Queue{}, err
	}

	return queues, nil
}

func (s *QueueService) UpdateQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
	queueName,
	queueDescription string,
) error {

	err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return err
	}

	_, _, err = s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	if err != nil {
		return err
	}

	err = s.repository.UpdateQueueDetails(
		ctx,
		queueID,
		queueName,
		queueDescription,
	)

	if err != nil {
		return err
	}
	notifyEvent(s.notifier, queueID, EventQueueUpdated, map[string]interface{}{
		"queue_id": queueID,
	})
	return nil
}

func (s *QueueService) UpdateAuthz(
	ctx context.Context,
	queueID,
	requesterID int64,
	queueAuthz model.QueueAuthz,
) (model.QueueAuthz, error) {

	err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return model.QueueAuthz{}, err
	}

	_, _, err = s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	if err != nil {
		return model.QueueAuthz{}, err
	}

	fresh, err := s.repository.UpdateQueueAuthz(
		ctx,
		queueID,
		queueAuthz,
	)
	if err != nil {
		return model.QueueAuthz{}, err
	}
	notifyEvent(s.notifier, queueID, EventQueueUpdated, map[string]interface{}{
		"queue_id":    queueID,
		"queue_authz": dto.NewQueueAuthzResponse(fresh),
	})
	return fresh, nil
}

func (s *QueueService) CloseQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) error {

	_, _, err := s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	if err != nil {
		return err
	}

	err = s.repository.CloseQueue(
		ctx,
		queueID,
	)
	if err != nil {
		return err
	}

	notifyEvent(s.notifier, queueID, EventQueueClosed, map[string]interface{}{
		"queue_id": queueID,
	})

	return nil
}

func (s *QueueService) isSubscribed(
	ctx context.Context,
	ownerID int64,
) error {

	isSubscribed, err := s.subscription.IsActive(
		ctx,
		ownerID,
	)

	if err != nil {
		return err
	}

	if !isSubscribed {
		return apperror.ErrSubscriptionInactive
	}
	return nil
}

func (s *QueueService) validateOwnership(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, model.QueueAuthz, error) {
	queue, queueAuthz, err := s.repository.GetQueueByID(
		ctx,
		queueID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}
	if requesterID != queue.OwnerID {
		return model.Queue{}, model.QueueAuthz{}, apperror.ErrNotFound
	}
	return queue, queueAuthz, nil
}
