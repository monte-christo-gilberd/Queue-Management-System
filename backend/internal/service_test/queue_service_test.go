package service_test

import (
	"context"
	"errors"
	"testing"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"qms/internal/service"

	"github.com/jmoiron/sqlx"
)

type fakeQueueRepo struct {
	createQueue model.Queue
	createAuthz model.QueueAuthz
	createErr   error

	getQueue model.Queue
	getAuthz model.QueueAuthz
	getErr   error

	listQueues []model.Queue
	listErr    error

	updateDetailsErr error
	updateAuthz      model.QueueAuthz
	updateAuthzErr   error
	closeErr         error
}

func (f *fakeQueueRepo) CreateNewQueue(
	ctx context.Context,
	tx *sqlx.Tx,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, model.QueueAuthz, error) {
	return f.createQueue, f.createAuthz, f.createErr
}

func (f *fakeQueueRepo) GetQueueByID(
	ctx context.Context,
	queueID int64,
) (model.Queue, model.QueueAuthz, error) {
	return f.getQueue, f.getAuthz, f.getErr
}

func (f *fakeQueueRepo) ListQueueByOwner(
	ctx context.Context,
	ownerID int64,
	activeOnly bool,
) ([]model.Queue, error) {
	return f.listQueues, f.listErr
}

func (f *fakeQueueRepo) UpdateQueueDetails(
	ctx context.Context,
	queueID int64,
	queueName,
	queueDescription string,
) error {
	return f.updateDetailsErr
}

func (f *fakeQueueRepo) UpdateQueueAuthz(
	ctx context.Context,
	queueID int64,
	authz model.QueueAuthz,
) (model.QueueAuthz, error) {
	if f.updateAuthzErr != nil {
		return model.QueueAuthz{}, f.updateAuthzErr
	}
	if f.updateAuthz.QueueID != 0 {
		return f.updateAuthz, nil
	}
	authz.QueueID = queueID
	return authz, nil
}

func (f *fakeQueueRepo) CloseQueue(
	ctx context.Context,
	queueID int64,
) error {
	return f.closeErr
}

type fakeSubscription struct {
	active bool
	err    error
}

func (f *fakeSubscription) IsActive(
	ctx context.Context,
	ownerID int64,
) (bool, error) {
	return f.active, f.err
}

type fakeNotifier struct {
	calls   int
	queueID int64
	action  string
	payload interface{}
}

func (f *fakeNotifier) Notify(
	queueID int64,
	action string,
	payload interface{},
) {
	f.calls++
	f.queueID = queueID
	f.action = action
	f.payload = payload
}

func TestCreateQueue_Success(t *testing.T) {
	ctx := context.Background()
	expectedQueue := model.Queue{ID: 1, OwnerID: 100, QueueName: "Antrian A"}
	expectedAuthz := model.QueueAuthz{QueueID: 1, CanManageQueue: true}

	repo := &fakeQueueRepo{
		createQueue: expectedQueue,
		createAuthz: expectedAuthz,
	}
	sub := &fakeSubscription{active: true}
	tx := &fakeTxRunner{}

	svc := service.NewQueueService(repo, sub, tx)

	q, authz, err := svc.CreateQueue(ctx, 100, "Antrian A", "Deskripsi")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if q.ID != expectedQueue.ID || authz.QueueID != expectedAuthz.QueueID {
		t.Errorf("mismatch return value: got (%+v, %+v), expected (%+v, %+v)", q, authz, expectedQueue, expectedAuthz)
	}
}

func TestCreateQueue_SubscriptionInactive(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{}
	sub := &fakeSubscription{active: false}
	tx := &fakeTxRunner{}

	svc := service.NewQueueService(repo, sub, tx)

	_, _, err := svc.CreateQueue(ctx, 100, "Antrian Baru", "Deskripsi")
	if !errors.Is(err, apperror.ErrSubscriptionInactive) {
		t.Errorf("expected ErrSubscriptionInactive, got: %v", err)
	}
}

func TestGetQueue_Success(t *testing.T) {
	ctx := context.Background()
	expectedQueue := model.Queue{ID: 1, OwnerID: 100, QueueName: "Antrian A"}
	expectedAuthz := model.QueueAuthz{QueueID: 1}

	repo := &fakeQueueRepo{
		getQueue: expectedQueue,
		getAuthz: expectedAuthz,
	}
	svc := service.NewQueueService(repo, nil, nil)

	q, authz, err := svc.GetQueue(ctx, 1, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if q.ID != expectedQueue.ID || authz.QueueID != expectedAuthz.QueueID {
		t.Errorf("mismatch result")
	}
}

func TestGetQueue_NotFound(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid Queue ID (Repo Error)", func(t *testing.T) {
		repo := &fakeQueueRepo{getErr: apperror.ErrNotFound}
		svc := service.NewQueueService(repo, nil, nil)

		_, _, err := svc.GetQueue(ctx, 999, 100)
		if !errors.Is(err, apperror.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})

	t.Run("Not Owner", func(t *testing.T) {
		repo := &fakeQueueRepo{
			getQueue: model.Queue{ID: 1, OwnerID: 100},
		}
		svc := service.NewQueueService(repo, nil, nil)

		// requesterID = 999 (Not Owner id 100)
		_, _, err := svc.GetQueue(ctx, 1, 999)
		if !errors.Is(err, apperror.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestListQueue_Success(t *testing.T) {
	ctx := context.Background()
	expectedList := []model.Queue{
		{ID: 1, OwnerID: 100, QueueName: "Antrian 1"},
		{ID: 2, OwnerID: 100, QueueName: "Antrian 2"},
	}

	repo := &fakeQueueRepo{listQueues: expectedList}
	sub := &fakeSubscription{active: true}
	svc := service.NewQueueService(repo, sub, nil)

	queues, err := svc.ListQueue(ctx, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(queues) != len(expectedList) {
		t.Errorf("expected %d queues, got %d", len(expectedList), len(queues))
	}
}

func TestUpdateQueue_Success(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 100},
	}
	sub := &fakeSubscription{active: true}
	svc := service.NewQueueService(repo, sub, nil)

	err := svc.UpdateQueue(ctx, 1, 100, "Name Updated", "Desc Updated")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateQueue_SubscriptionInactive(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{}
	sub := &fakeSubscription{active: false}
	svc := service.NewQueueService(repo, sub, nil)

	err := svc.UpdateQueue(ctx, 1, 100, "Name", "Desc")
	if !errors.Is(err, apperror.ErrSubscriptionInactive) {
		t.Errorf("expected ErrSubscriptionInactive, got: %v", err)
	}
}

func TestUpdateQueue_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{getErr: apperror.ErrNotFound}
	sub := &fakeSubscription{active: true}
	svc := service.NewQueueService(repo, sub, nil)

	err := svc.UpdateQueue(ctx, 999, 100, "Name", "Desc")
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestUpdateAuthz_Success(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 100},
	}
	sub := &fakeSubscription{active: true}
	svc := service.NewQueueService(repo, sub, nil)

	newAuthz := model.QueueAuthz{QueueID: 1, CanManageQueue: true}
	fresh, err := svc.UpdateAuthz(ctx, 1, 100, newAuthz)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if fresh.QueueID != 1 {
		t.Errorf("expected fresh QueueID 1, got %d", fresh.QueueID)
	}
}

func TestUpdateAuthz_NotifiesQueueUpdated(t *testing.T) {
	ctx := context.Background()
	freshAuthz := model.QueueAuthz{QueueID: 1, CanManageQueue: true, CanViewQueue: true}
	repo := &fakeQueueRepo{
		getQueue:    model.Queue{ID: 1, OwnerID: 100},
		updateAuthz: freshAuthz,
	}
	sub := &fakeSubscription{active: true}
	notifier := &fakeNotifier{}
	svc := service.NewQueueService(repo, sub, nil, notifier)

	newAuthz := model.QueueAuthz{QueueID: 1, CanManageQueue: true}
	got, err := svc.UpdateAuthz(ctx, 1, 100, newAuthz)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != freshAuthz {
		t.Errorf("expected fresh %+v, got %+v", freshAuthz, got)
	}

	if notifier.calls != 1 {
		t.Fatalf("expected 1 notify call, got %d", notifier.calls)
	}
	if notifier.queueID != 1 {
		t.Errorf("expected queueID 1, got %d", notifier.queueID)
	}
	if notifier.action != service.EventQueueUpdated {
		t.Errorf("expected action %q, got %q", service.EventQueueUpdated, notifier.action)
	}
	payload, ok := notifier.payload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map payload, got %T", notifier.payload)
	}
	if payload["queue_authz"] != dto.NewQueueAuthzResponse(freshAuthz) {
		t.Errorf("expected payload authz %+v, got %+v", freshAuthz, payload["queue_authz"])
	}
}

func TestUpdateAuthz_NoNotifyOnRepoError(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{
		getQueue:       model.Queue{ID: 1, OwnerID: 100},
		updateAuthzErr: apperror.ErrNotFound,
	}
	sub := &fakeSubscription{active: true}
	notifier := &fakeNotifier{}
	svc := service.NewQueueService(repo, sub, nil, notifier)

	if _, err := svc.UpdateAuthz(ctx, 1, 100, model.QueueAuthz{}); err == nil {
		t.Fatal("expected error, got nil")
	}
	if notifier.calls != 0 {
		t.Errorf("expected 0 notify calls on repo error, got %d", notifier.calls)
	}
}

func TestUpdateAuthz_ReturnsFreshFromRepo(t *testing.T) {
	ctx := context.Background()
	freshAuthz := model.QueueAuthz{QueueID: 1, CanManageQueue: true, CanManageNotes: true}
	repo := &fakeQueueRepo{
		getQueue:    model.Queue{ID: 1, OwnerID: 100},
		updateAuthz: freshAuthz,
	}
	sub := &fakeSubscription{active: true}
	svc := service.NewQueueService(repo, sub, nil)

	got, err := svc.UpdateAuthz(ctx, 1, 100, model.QueueAuthz{CanManageQueue: true})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != freshAuthz {
		t.Errorf("expected fresh %+v, got %+v", freshAuthz, got)
	}
}

func TestUpdateAuthz_SubscriptionInactive(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{}
	sub := &fakeSubscription{active: false}
	svc := service.NewQueueService(repo, sub, nil)

	_, err := svc.UpdateAuthz(ctx, 1, 100, model.QueueAuthz{})
	if !errors.Is(err, apperror.ErrSubscriptionInactive) {
		t.Errorf("expected ErrSubscriptionInactive, got: %v", err)
	}
}

func TestUpdateAuthz_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{getErr: apperror.ErrNotFound}
	sub := &fakeSubscription{active: true}
	svc := service.NewQueueService(repo, sub, nil)

	_, err := svc.UpdateAuthz(ctx, 999, 100, model.QueueAuthz{})
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestCloseQueue_Success(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 100},
	}
	svc := service.NewQueueService(repo, nil, nil)

	err := svc.CloseQueue(ctx, 1, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCloseQueue_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{getErr: apperror.ErrNotFound}
	svc := service.NewQueueService(repo, nil, nil)

	err := svc.CloseQueue(ctx, 999, 100)
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

type fakeHolders struct {
	has bool
	err error
}

func (f fakeHolders) HasTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (bool, error) {
	return f.has, f.err
}

func TestViewQueue_OwnerGetsAuthz(t *testing.T) {
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 7},
		getAuthz: model.QueueAuthz{QueueID: 1, CanManageQueue: true},
	}
	svc := service.NewQueueService(repo, nil, nil).
		WithTicketHolderChecker(fakeHolders{})

	_, authz, isOwner, err := svc.ViewQueue(context.Background(), 1, 7)

	if err != nil || !isOwner || !authz.CanManageQueue {
		t.Fatalf("owner: err=%v isOwner=%v authz=%+v", err, isOwner, authz)
	}
}

func TestViewQueue_TicketHolderGetsQueueWithoutAuthz(t *testing.T) {
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 7},
		getAuthz: model.QueueAuthz{QueueID: 1, CanManageQueue: true},
	}
	svc := service.NewQueueService(repo, nil, nil).
		WithTicketHolderChecker(fakeHolders{has: true})

	queue, authz, isOwner, err := svc.ViewQueue(context.Background(), 1, 8)

	if err != nil || isOwner || queue.ID != 1 || authz != (model.QueueAuthz{}) {
		t.Fatalf("holder: err=%v isOwner=%v queue=%+v authz=%+v", err, isOwner, queue, authz)
	}
}

func TestViewQueue_OutsiderIsNotFound(t *testing.T) {
	repo := &fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 7}}
	svc := service.NewQueueService(repo, nil, nil).
		WithTicketHolderChecker(fakeHolders{has: false})

	_, _, _, err := svc.ViewQueue(context.Background(), 1, 8)

	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestViewQueue_NoHolderCheckerMeansOwnerOnly(t *testing.T) {
	repo := &fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 7}}
	svc := service.NewQueueService(repo, nil, nil)

	_, _, _, err := svc.ViewQueue(context.Background(), 1, 8)

	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetActiveQueue(t *testing.T) {
	cases := []struct {
		name    string
		repo    *fakeQueueRepo
		wantErr error
	}{
		{"active", &fakeQueueRepo{getQueue: model.Queue{ID: 1, Status: "active"}}, nil},
		{"closed", &fakeQueueRepo{getQueue: model.Queue{ID: 1, Status: "closed"}}, apperror.ErrQueueClosed},
		{"missing", &fakeQueueRepo{getErr: apperror.ErrNotFound}, apperror.ErrNotFound},
	}
	for _, c := range cases {
		svc := service.NewQueueService(c.repo, nil, nil)

		_, err := svc.GetActiveQueue(context.Background(), 1)

		if !errors.Is(err, c.wantErr) {
			t.Errorf("%s: expected %v, got %v", c.name, c.wantErr, err)
		}
	}
}
