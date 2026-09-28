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

type fakeTxRunner struct{}

func (fakeTxRunner) WithTx(
	ctx context.Context,
	fn func(*sqlx.Tx) error,
) error {
	return fn(nil)
}

func TestJoinQueue_RetriesOnDuplicate(t *testing.T) {
	repo := &fakeTicketRepo{failTimes: 2}
	svc := service.NewTicketService(repo, fakeTxRunner{})

	_, err := svc.JoinQueue(context.Background(), 1, 99)

	assert.NoError(t, err)
	assert.Equal(t, 3, repo.insertCallCount)
}

func TestJoinQueue_GivesUpAfterMaxRetries(t *testing.T) {
	repo := &fakeTicketRepo{failTimes: 999}
	svc := service.NewTicketService(repo, fakeTxRunner{})

	_, err := svc.JoinQueue(context.Background(), 1, 1)

	assert.ErrorIs(t, err, apperror.ErrRetryExhausted)
}
