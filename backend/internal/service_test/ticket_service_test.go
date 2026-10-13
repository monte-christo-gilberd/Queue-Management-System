package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
)

type fakeTicketRepo struct {
	insertCallCount int
	failTimes       int // gagal N kali dulu, baru sukses

	listResult   []model.Ticket
	listCalled   bool
	listStatuses []string

	alreadyJoined      bool // HasTicket true sejak awal
	joinedAfterInserts int  // HasTicket jadi true setelah N kali Insert (simulasi balapan)
}

func (f *fakeTicketRepo) MaxWaitingSortKey(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (int64, error) {
	return 0, nil // untuk test ini, nilainya tidak terlalu penting
}

func (f *fakeTicketRepo) Insert(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	userID,
	sortKey int64,
) (model.Ticket, error) {
	f.insertCallCount++

	if f.insertCallCount <= f.failTimes {
		return model.Ticket{}, apperror.ErrDuplicate
	}

	return model.Ticket{}, nil
}

func (f *fakeTicketRepo) NextWaiting(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.Ticket, error) {
	return model.Ticket{}, nil // tidak dipakai di test ini, isi kosong saja
}

func (f *fakeTicketRepo) MarkCompleted(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {
	return nil // tidak dipakai di test ini
}


func (f *fakeTicketRepo) HasTicket(
	ctx context.Context,
	queueID,
	userID int64,
) (bool, error) {
	if f.alreadyJoined {
		return true, nil
	}
	return f.joinedAfterInserts > 0 && f.insertCallCount >= f.joinedAfterInserts, nil
}

func (f *fakeTicketRepo) ListByQueue(
	ctx context.Context,
	queueID int64,
	statuses []string,
) ([]model.Ticket, error) {
	f.listCalled = true
	f.listStatuses = statuses
	return f.listResult, nil
}

type fakeQueueGuard struct {
	ownerID int64
	closed  bool
	missing bool
}

func (g fakeQueueGuard) GetActiveQueue(
	ctx context.Context,
	queueID int64,
) (model.Queue, error) {
	if g.missing {
		return model.Queue{}, apperror.ErrNotFound
	}
	if g.closed {
		return model.Queue{}, apperror.ErrQueueClosed
	}
	return model.Queue{ID: queueID, OwnerID: g.ownerID, Status: "active"}, nil
}

func (g fakeQueueGuard) GetQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, model.QueueAuthz, error) {
	if requesterID != g.ownerID {
		return model.Queue{}, model.QueueAuthz{}, apperror.ErrNotFound
	}
	return model.Queue{ID: queueID, OwnerID: g.ownerID}, model.QueueAuthz{}, nil
}

func TestListTickets_OwnerGetsTicketsFromRepo(t *testing.T) {
	want := []model.Ticket{{ID: 1, TicketNumber: 1}, {ID: 2, TicketNumber: 2}}
	repo := &fakeTicketRepo{listResult: want}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	got, err := svc.ListTickets(context.Background(), 1, 7, []string{"waiting"})

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, []string{"waiting"}, repo.listStatuses)
}

func TestListTickets_NonOwnerGetsNotFoundAndRepoUntouched(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.ListTickets(context.Background(), 1, 8, nil)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.False(t, repo.listCalled)
}

func TestListTickets_ErrorsWithoutGuard(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{})

	_, err := svc.ListTickets(context.Background(), 1, 7, nil)

	assert.Error(t, err)
	assert.False(t, repo.listCalled)
}

func TestJoinQueue_RetriesOnDuplicate(t *testing.T) {
	repo := &fakeTicketRepo{failTimes: 2}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.NoError(t, err)
	assert.Equal(t, 3, repo.insertCallCount)
}

func TestJoinQueue_GivesUpAfterMaxRetries(t *testing.T) {
	repo := &fakeTicketRepo{failTimes: 999}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.JoinQueue(context.Background(), 1, 1)

	assert.ErrorIs(t, err, apperror.ErrRetryExhausted)
}

func TestJoinQueue_ClosedQueueIsRejectedWithoutInsert(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7, closed: true})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.ErrorIs(t, err, apperror.ErrQueueClosed)
	assert.Equal(t, 0, repo.insertCallCount)
}

func TestJoinQueue_MissingQueueIsNotFound(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7, missing: true})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, repo.insertCallCount)
}

func TestJoinQueue_ErrorsWithoutGuard(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.Error(t, err)
	assert.Equal(t, 0, repo.insertCallCount)
}

func TestNextTicket_OwnerSucceeds(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.NextTicket(context.Background(), 1, 7)

	assert.NoError(t, err)
}

func TestNextTicket_NonOwnerIsNotFound(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.NextTicket(context.Background(), 1, 8)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestNextTicket_ErrorsWithoutGuard(t *testing.T) {
	svc := service.NewTicketService(&fakeTicketRepo{}, fakeTxRunner{})

	_, err := svc.NextTicket(context.Background(), 1, 7)

	assert.Error(t, err)
}

func TestJoinQueue_AlreadyJoinedIsRejectedWithoutInsert(t *testing.T) {
	repo := &fakeTicketRepo{alreadyJoined: true}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.ErrorIs(t, err, apperror.ErrAlreadyJoined)
	assert.Equal(t, 0, repo.insertCallCount)
}

func TestJoinQueue_ConcurrentSameUserDuplicateBecomesAlreadyJoined(t *testing.T) {
	// Insert pertama gagal duplikat karena join lain milik user yang sama baru saja commit.
	repo := &fakeTicketRepo{failTimes: 1, joinedAfterInserts: 1}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.ErrorIs(t, err, apperror.ErrAlreadyJoined)
	assert.Equal(t, 1, repo.insertCallCount)
}
