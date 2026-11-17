package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"

	"github.com/jmoiron/sqlx"
)

type fakeQueueRepo struct {
	createQueue model.Queue
	createErr   error

	getQueue model.Queue
	getErr   error

	listQueues []model.Queue
	listErr    error

	updateDetailsErr error
	closeErr         error
}

func (f *fakeQueueRepo) CreateNewQueue(
	ctx context.Context,
	tx *sqlx.Tx,
	ownerID int64,
	queueName,
	queueDescription string,
) (model.Queue, error) {
	return f.createQueue, f.createErr
}

func (f *fakeQueueRepo) GetQueueByID(
	ctx context.Context,
	queueID int64,
) (model.Queue, error) {
	return f.getQueue, f.getErr
}

func (f *fakeQueueRepo) ListQueueByOwner(
	ctx context.Context,
	ownerID int64,
	activeOnly bool,
) ([]model.Queue, error) {
	return f.listQueues, f.listErr
}

func (f *fakeQueueRepo) SetTicketPrefix(
	ctx context.Context,
	queueID int64,
	prefix string,
) error {
	return f.updateDetailsErr
}

func (f *fakeQueueRepo) UpdateQueueDetails(
	ctx context.Context,
	queueID int64,
	queueName,
	queueDescription string,
) error {
	return f.updateDetailsErr
}

func (f *fakeQueueRepo) CloseQueue(
	ctx context.Context,
	queueID int64,
) error {
	return f.closeErr
}

func (f *fakeQueueRepo) GetQueueSettings(
	ctx context.Context,
	queueID int64,
) (model.QueueSettings, error) {
	return model.QueueSettings{}, nil
}

func (f *fakeQueueRepo) CreateQueueSettingsIfMissing(
	ctx context.Context,
	queueID int64,
) error {
	return nil
}

func (f *fakeQueueRepo) GetSkipSettings(
	ctx context.Context,
	queueID int64,
) (model.SkipSettings, error) {
	return model.DefaultSkipSettings(), nil
}

func (f *fakeQueueRepo) UpdateSkipSettings(
	ctx context.Context,
	queueID int64,
	settings model.SkipSettings,
) error {
	return nil
}

func (f *fakeQueueRepo) UpdateQRType(
	ctx context.Context,
	queueID int64,
	qrType string,
) error {
	return nil
}

func (f *fakeQueueRepo) UpdateQRCurrentCode(
	ctx context.Context,
	queueID int64,
	token string,
) error {
	return nil
}

// GetQueueSettingsByJoinCode: stub; selalu tidak ditemukan.
func (f *fakeQueueRepo) GetQueueSettingsByJoinCode(
	ctx context.Context,
	code string,
) (model.QueueSettings, error) {
	return model.QueueSettings{}, apperror.ErrNotFound
}

// UpdateDynamicQRCode: stub; tidak menyimpan apa pun.
func (f *fakeQueueRepo) UpdateDynamicQRCode(
	ctx context.Context,
	queueID int64,
	code string,
	expiresAt time.Time,
) error {
	return nil
}

// RotateDynamicQRCode: stub; selalu melaporkan kode tidak berganti.
func (f *fakeQueueRepo) RotateDynamicQRCode(
	ctx context.Context,
	queueID int64,
	oldCode string,
	newCode string,
	expiresAt time.Time,
) (bool, error) {
	return false, nil
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

	repo := &fakeQueueRepo{
		createQueue: expectedQueue,
	}
	sub := &fakeSubscription{active: true}
	tx := &fakeTxRunner{}

	svc := service.NewQueueService(repo, sub, tx)

	q, err := svc.CreateQueue(ctx, 100, "Antrian A", "Deskripsi")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if q.ID != expectedQueue.ID {
		t.Errorf("mismatch return value: got %+v, expected %+v", q, expectedQueue)
	}
}

func TestCreateQueue_SubscriptionInactive(t *testing.T) {
	ctx := context.Background()
	repo := &fakeQueueRepo{}
	sub := &fakeSubscription{active: false}
	tx := &fakeTxRunner{}

	svc := service.NewQueueService(repo, sub, tx)

	_, err := svc.CreateQueue(ctx, 100, "Antrian Baru", "Deskripsi")
	if !errors.Is(err, apperror.ErrSubscriptionInactive) {
		t.Errorf("expected ErrSubscriptionInactive, got: %v", err)
	}
}

func TestGetQueue_Success(t *testing.T) {
	ctx := context.Background()
	expectedQueue := model.Queue{ID: 1, OwnerID: 100, QueueName: "Antrian A"}

	repo := &fakeQueueRepo{
		getQueue: expectedQueue,
	}
	svc := service.NewQueueService(repo, nil, nil)

	q, err := svc.GetQueue(ctx, 1, 100)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if q.ID != expectedQueue.ID {
		t.Errorf("mismatch result")
	}
}

func TestGetQueue_NotFound(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid Queue ID (Repo Error)", func(t *testing.T) {
		repo := &fakeQueueRepo{getErr: apperror.ErrNotFound}
		svc := service.NewQueueService(repo, nil, nil)

		_, err := svc.GetQueue(ctx, 999, 100)
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
		_, err := svc.GetQueue(ctx, 1, 999)
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

func (f fakeHolders) HasAnyTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (bool, error) {
	return f.has, f.err
}

func TestViewQueue_OwnerGetsQueue(t *testing.T) {
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 7},
	}
	svc := service.NewQueueService(repo, nil, nil).
		WithTicketHolderChecker(fakeHolders{})

	queue, isOwner, err := svc.ViewQueue(context.Background(), 1, 7)

	if err != nil || !isOwner || queue.ID != 1 {
		t.Fatalf("owner: err=%v isOwner=%v queue=%+v", err, isOwner, queue)
	}
}

func TestViewQueue_TicketHolderGetsQueueWithoutOwnerFlag(t *testing.T) {
	repo := &fakeQueueRepo{
		getQueue: model.Queue{ID: 1, OwnerID: 7},
	}
	svc := service.NewQueueService(repo, nil, nil).
		WithTicketHolderChecker(fakeHolders{has: true})

	queue, isOwner, err := svc.ViewQueue(context.Background(), 1, 8)

	if err != nil || isOwner || queue.ID != 1 {
		t.Fatalf("holder: err=%v isOwner=%v queue=%+v", err, isOwner, queue)
	}
}

func TestViewQueue_OutsiderIsNotFound(t *testing.T) {
	repo := &fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 7}}
	svc := service.NewQueueService(repo, nil, nil).
		WithTicketHolderChecker(fakeHolders{has: false})

	_, _, err := svc.ViewQueue(context.Background(), 1, 8)

	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestViewQueue_NoHolderCheckerMeansOwnerOnly(t *testing.T) {
	repo := &fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 7}}
	svc := service.NewQueueService(repo, nil, nil)

	_, _, err := svc.ViewQueue(context.Background(), 1, 8)

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

func TestUpdateSkipSettings_Validation(t *testing.T) {
	ctx := context.Background()
	open := model.Queue{ID: 1, OwnerID: 100, Status: "active"}
	cases := []struct {
		name string
		in   model.SkipSettings
		ok   bool
	}{
		{"remove", model.SkipSettings{Fallback: "remove", MaxReturns: 1}, true},
		{"back", model.SkipSettings{Fallback: "back", MaxReturns: 2}, true},
		{"after_n", model.SkipSettings{Fallback: "after_n", N: 3, MaxReturns: 1}, true},
		{"after_n zero", model.SkipSettings{Fallback: "after_n", N: 0, MaxReturns: 1}, false},
		{"unknown", model.SkipSettings{Fallback: "x", MaxReturns: 1}, false},
		{"max zero", model.SkipSettings{Fallback: "back", MaxReturns: 0}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := service.NewQueueService(&fakeQueueRepo{getQueue: open}, &fakeSubscription{active: true}, &fakeTxRunner{})
			_, err := svc.UpdateSkipSettings(ctx, 1, 100, c.in)
			if c.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !c.ok && !errors.Is(err, apperror.ErrInvalidSkipSettings) {
				t.Fatalf("expected ErrInvalidSkipSettings, got %v", err)
			}
		})
	}
}

func TestUpdateSkipSettings_Guards(t *testing.T) {
	ctx := context.Background()
	in := model.SkipSettings{Fallback: "back", MaxReturns: 1}

	svc := service.NewQueueService(&fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 100}}, &fakeSubscription{active: true}, &fakeTxRunner{})
	if _, err := svc.UpdateSkipSettings(ctx, 1, 999, in); !errors.Is(err, apperror.ErrNotFound) {
		t.Errorf("non-owner: expected ErrNotFound, got %v", err)
	}

	svc = service.NewQueueService(&fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 100, Status: "closed"}}, &fakeSubscription{active: true}, &fakeTxRunner{})
	if _, err := svc.UpdateSkipSettings(ctx, 1, 100, in); !errors.Is(err, apperror.ErrQueueClosed) {
		t.Errorf("closed: expected ErrQueueClosed, got %v", err)
	}

	svc = service.NewQueueService(&fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 100, Status: "active"}}, &fakeSubscription{active: false}, &fakeTxRunner{})
	if _, err := svc.UpdateSkipSettings(ctx, 1, 100, in); !errors.Is(err, apperror.ErrSubscriptionInactive) {
		t.Errorf("inactive: expected ErrSubscriptionInactive, got %v", err)
	}

	svc = service.NewQueueService(&fakeQueueRepo{getQueue: model.Queue{ID: 1, OwnerID: 100}}, nil, nil)
	if _, err := svc.GetSkipSettings(ctx, 1, 999); !errors.Is(err, apperror.ErrNotFound) {
		t.Errorf("get non-owner: expected ErrNotFound, got %v", err)
	}
}
