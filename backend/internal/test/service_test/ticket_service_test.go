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

	myTicket        *model.Ticket // hasil FindByUser; nil = ErrNotFound
	lastCompleted   int
	hasCompleted    bool
	waitingBefore   int
	waitingBeforeSK int64 // sort_key yang diterima CountWaitingBefore
	countCalled     bool

	userTickets []model.UserTicket

	recentNumbers []int
	firstWaiting  int
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
	counterID int64,
) error {
	return nil // tidak dipakai di test ini
}

func (f *fakeTicketRepo) CurrentServing(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.Ticket, error) {
	return model.Ticket{}, apperror.ErrNotFound
}

func (f *fakeTicketRepo) MarkRecalled(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {
	return nil
}

func (f *fakeTicketRepo) MarkSkipped(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID int64,
) error {
	return nil
}

func (f *fakeTicketRepo) SkipSettings(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
) (model.SkipSettings, error) {
	return model.DefaultSkipSettings(), nil
}

func (f *fakeTicketRepo) WaitingSortKeys(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID int64,
	limit int,
) ([]int64, error) {
	return nil, nil
}

func (f *fakeTicketRepo) SortKeyTaken(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	sortKey int64,
) (bool, error) {
	return false, nil
}

func (f *fakeTicketRepo) RequeueSkipped(
	ctx context.Context,
	tx *sqlx.Tx,
	ticketID,
	sortKey int64,
) error {
	return nil
}

func (f *fakeTicketRepo) RebalanceWaiting(
	ctx context.Context,
	tx *sqlx.Tx,
	queueID,
	gap int64,
) error {
	return nil
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

func (f *fakeTicketRepo) FindByUser(
	ctx context.Context,
	queueID,
	userID int64,
) (model.Ticket, error) {
	if f.myTicket == nil {
		return model.Ticket{}, apperror.ErrNotFound
	}
	return *f.myTicket, nil
}

func (f *fakeTicketRepo) LastCompletedNumber(
	ctx context.Context,
	queueID int64,
) (int, bool, error) {
	return f.lastCompleted, f.hasCompleted, nil
}

func (f *fakeTicketRepo) ListByUser(
	ctx context.Context,
	userID int64,
) ([]model.UserTicket, error) {
	return f.userTickets, nil
}

func (f *fakeTicketRepo) RecentCompletedNumbers(
	ctx context.Context,
	queueID int64,
	n int,
) ([]int, error) {
	return f.recentNumbers, nil
}

func (f *fakeTicketRepo) FirstWaitingNumber(
	ctx context.Context,
	queueID int64,
) (int, bool, error) {
	return f.firstWaiting, f.firstWaiting != 0, nil
}

func (f *fakeTicketRepo) CountWaitingBefore(
	ctx context.Context,
	queueID,
	sortKey int64,
) (int, error) {
	f.countCalled = true
	f.waitingBeforeSK = sortKey
	return f.waitingBefore, nil
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

func (f *fakeTicketRepo) GetCounterByQueue(
	ctx context.Context,
	queueID int64,
) (model.Counter, error) {
	return model.Counter{ID: 1, QueueID: queueID, Name: "Counter 1", IsDefault: true}, nil
}

type fakeQueueGuard struct {
	ownerID int64
	closed  bool
	missing bool
}

// ViewQueue: hanya owner dan userID 55 (pemegang tiket di tes) yang boleh.
func (g fakeQueueGuard) ViewQueue(
	ctx context.Context,
	queueID,
	requesterID int64,
) (model.Queue, bool, error) {
	if requesterID == g.ownerID {
		return model.Queue{ID: queueID, OwnerID: g.ownerID}, true, nil
	}
	if requesterID == 55 {
		return model.Queue{ID: queueID, OwnerID: g.ownerID}, false, nil
	}
	return model.Queue{}, false, apperror.ErrNotFound
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
) (model.Queue, error) {
	if requesterID != g.ownerID {
		return model.Queue{}, apperror.ErrNotFound
	}
	return model.Queue{ID: queueID, OwnerID: g.ownerID}, nil
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

	_, err := svc.NextTicket(context.Background(), 1, 7, 0)

	assert.NoError(t, err)
}

func TestNextTicket_NonOwnerIsNotFound(t *testing.T) {
	repo := &fakeTicketRepo{}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	_, err := svc.NextTicket(context.Background(), 1, 8, 0)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestNextTicket_ErrorsWithoutGuard(t *testing.T) {
	svc := service.NewTicketService(&fakeTicketRepo{}, fakeTxRunner{})

	_, err := svc.NextTicket(context.Background(), 1, 7, 0)

	assert.Error(t, err)
}

func TestJoinQueue_AlreadyJoinedReturnsExistingTicketWithoutInsert(t *testing.T) {
	// Spek 4.2 / TC-05-04: join ulang menampilkan nomor yang sudah ada, tanpa tiket kedua.
	existing := model.Ticket{ID: 5, QueueID: 1, UserID: 99, TicketNumber: 3, Status: "waiting"}
	repo := &fakeTicketRepo{alreadyJoined: true, myTicket: &existing}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	got, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.NoError(t, err)
	assert.Equal(t, existing, got)
	assert.Equal(t, 0, repo.insertCallCount)
}

func TestJoinQueue_ConcurrentSameUserDuplicateReturnsWinnerTicket(t *testing.T) {
	// Insert pertama gagal duplikat karena join lain milik user yang sama baru saja commit:
	// hasilnya tiket pemenang, bukan error dan bukan tiket kedua.
	winner := model.Ticket{ID: 9, QueueID: 1, UserID: 99, TicketNumber: 1, Status: "waiting"}
	repo := &fakeTicketRepo{failTimes: 1, joinedAfterInserts: 1, myTicket: &winner}
	svc := service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})

	got, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.NoError(t, err)
	assert.Equal(t, winner.ID, got.ID)
	assert.Equal(t, 1, repo.insertCallCount)
}

func newStatusSvc(repo *fakeTicketRepo) *service.TicketService {
	return service.NewTicketService(repo, fakeTxRunner{}).
		WithQueueGuard(fakeQueueGuard{ownerID: 7})
}

func TestQueueStatus_WaitingHolderGetsAheadCountAndCurrent(t *testing.T) {
	repo := &fakeTicketRepo{
		myTicket:      &model.Ticket{ID: 9, TicketNumber: 5, SortKey: 327680, Status: "waiting"},
		lastCompleted: 3, hasCompleted: true,
		waitingBefore: 1,
	}

	got, err := newStatusSvc(repo).QueueStatus(context.Background(), 1, 55)

	assert.NoError(t, err)
	if assert.NotNil(t, got.MyTicket) {
		assert.Equal(t, 5, got.MyTicket.TicketNumber)
	}
	if assert.NotNil(t, got.CurrentNumber) {
		assert.Equal(t, 3, *got.CurrentNumber)
	}
	if assert.NotNil(t, got.WaitingAhead) {
		assert.Equal(t, 1, *got.WaitingAhead)
	}
	assert.Equal(t, int64(327680), repo.waitingBeforeSK)
}

func TestQueueStatus_CompletedHolderHasNoAheadCount(t *testing.T) {
	repo := &fakeTicketRepo{
		myTicket:      &model.Ticket{ID: 9, TicketNumber: 2, SortKey: 131072, Status: "completed"},
		lastCompleted: 4, hasCompleted: true,
	}

	got, err := newStatusSvc(repo).QueueStatus(context.Background(), 1, 55)

	assert.NoError(t, err)
	assert.NotNil(t, got.MyTicket)
	assert.Nil(t, got.WaitingAhead)
	assert.False(t, repo.countCalled)
}

func TestQueueStatus_OwnerWithoutTicketAndNothingServedYet(t *testing.T) {
	repo := &fakeTicketRepo{}

	got, err := newStatusSvc(repo).QueueStatus(context.Background(), 1, 7)

	assert.NoError(t, err)
	assert.Nil(t, got.MyTicket)
	assert.Nil(t, got.CurrentNumber)
	assert.Nil(t, got.WaitingAhead)
}

func TestQueueStatus_OutsiderIsNotFound(t *testing.T) {
	_, err := newStatusSvc(&fakeTicketRepo{}).QueueStatus(context.Background(), 1, 99)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestQueueStatus_ErrorsWithoutGuard(t *testing.T) {
	svc := service.NewTicketService(&fakeTicketRepo{}, fakeTxRunner{})

	_, err := svc.QueueStatus(context.Background(), 1, 7)

	assert.Error(t, err)
}

func TestQueueStatus_PreviousAndNextNumbers(t *testing.T) {
	repo := &fakeTicketRepo{
		myTicket:      &model.Ticket{ID: 9, TicketNumber: 5, SortKey: 327680, Status: "waiting"},
		lastCompleted: 2, hasCompleted: true,
		recentNumbers: []int{2, 1},
		firstWaiting:  3,
	}

	got, err := newStatusSvc(repo).QueueStatus(context.Background(), 1, 55)

	assert.NoError(t, err)
	if assert.NotNil(t, got.PreviousNumber) {
		assert.Equal(t, 1, *got.PreviousNumber)
	}
	if assert.NotNil(t, got.NextNumber) {
		assert.Equal(t, 3, *got.NextNumber)
	}
}
