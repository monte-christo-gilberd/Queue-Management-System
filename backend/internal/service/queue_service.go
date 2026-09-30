package service

import (
	"context"
	"qms/internal/apperror"
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
	) error
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

type QueueService struct {
	repository   QueueRepository
	subscription SubscriptionChecker
	tx           TxRunner
}

func NewQueueService(
	repository QueueRepository,
	subscription SubscriptionChecker,
	tx TxRunner,
) *QueueService {
	return &QueueService{
		repository:   repository,
		subscription: subscription,
		tx:           tx,
	}
}

func (s *QueueService) CreateQueue(
	ctx context.Context,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, model.QueueAuthz, error) {

	subscribed, err := s.isSubscribed(
		ctx,
		ownerID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	if !subscribed {
		return model.Queue{}, model.QueueAuthz{}, apperror.ErrSubscriptionInactive
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

	subscribed, err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return model.Queue{}, model.QueueAuthz{}, err
	}

	if !subscribed {
		return model.Queue{}, model.QueueAuthz{}, apperror.ErrSubscriptionInactive
	}

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

func (s *QueueService) ListQueue(
	ctx context.Context,
	requesterID int64,
) ([]model.Queue, error) {

	subscribed, err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return []model.Queue{}, err
	}

	if !subscribed {
		return []model.Queue{}, apperror.ErrSubscriptionInactive
	}

	queues, err := s.repository.ListQueueByOwner(
		ctx,
		requesterID,
	)
	if err != nil {
		return []model.Queue{}, err
	}

	if queues == nil {
		return []model.Queue{}, apperror.ErrNotFound
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

	subscribed, err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return err
	}

	if !subscribed {
		return apperror.ErrSubscriptionInactive
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
	return nil
}

func (s *QueueService) UpdateAuthz(
	ctx context.Context,
	queueID,
	requesterID int64,
	queueAuthz model.QueueAuthz,
) error {

	subscribed, err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return err
	}

	if !subscribed {
		return apperror.ErrSubscriptionInactive
	}

	_, _, err = s.validateOwnership(
		ctx,
		queueID,
		requesterID,
	)
	if err != nil {
		return err
	}

	err = s.repository.UpdateQueueAuthz(
		ctx,
		queueID,
		queueAuthz,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *QueueService) CloseQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) error {

	subscribed, err := s.isSubscribed(
		ctx,
		requesterID,
	)
	if err != nil {
		return err
	}

	if !subscribed {
		return apperror.ErrSubscriptionInactive
	}

	_, _, err = s.validateOwnership(
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

	return nil
}

func (s *QueueService) isSubscribed(
	ctx context.Context,
	ownerID int64,
) (bool, error) {

	isSubscribed, err := s.subscription.IsActive(
		ctx,
		ownerID,
	)
	if err != nil {
		return false, err
	}

	if isSubscribed {
		return true, nil
	}
	return false, nil
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
