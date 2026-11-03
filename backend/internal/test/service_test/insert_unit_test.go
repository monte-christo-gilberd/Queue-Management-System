package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAccounts: fake service.AccountFinder. users adalah hasil pencarian untuk identifier apa pun.
type fakeAccounts struct {
	users []model.User
	err   error
}

func (f fakeAccounts) FindByLogin(_ context.Context, _ string) ([]model.User, error) {
	return f.users, f.err
}

func newInsertSvc(env *specEnv, accounts service.AccountFinder) *service.TicketService {
	return service.NewTicketService(env.tickets, fakeTxRunner{}, env.events).
		WithQueueGuard(env.qs).
		WithAccountFinder(accounts)
}

// waitingUserIDs: urutan pemilik tiket waiting menurut sort_key (urutan tunggu sebenarnya).
func waitingUserIDs(env *specEnv) []int64 {
	var ids []int64
	for _, t := range env.tickets.waitingInOrder(env.queueID) {
		ids = append(ids, t.UserID)
	}
	return ids
}

func oneUser(id int64) fakeAccounts {
	return fakeAccounts{users: []model.User{{ID: id}}}
}

func TestInsert_MiddlePositionShiftsTheRestBack(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)
	joinedBefore := env.events.count(service.EventTicketJoined)

	got, err := newInsertSvc(env, oneUser(specCustomerD)).InsertTicket(ctx, env.queueID, specOwnerID, "d@test.com", 2)

	require.NoError(t, err)
	assert.Equal(t, "waiting", got.Status)
	assert.Equal(t, []int64{specCustomerA, specCustomerD, specCustomerB, specCustomerC}, waitingUserIDs(env))
	assert.Equal(t, joinedBefore+1, env.events.count(service.EventTicketJoined), "sisip memancarkan ticket_joined")
}

func TestInsert_PositionOneWhileServingBecomesNextNotCurrent(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = newInsertSvc(env, oneUser(specCustomerC)).InsertTicket(ctx, env.queueID, specOwnerID, "c@test.com", 1)

	require.NoError(t, err)
	assert.Equal(t, []int64{specCustomerC, specCustomerB}, waitingUserIDs(env), "sisipan menjadi berikutnya")
	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 1, *st.CurrentNumber, "nomor yang sedang dilayani tidak terganggu")
}

func TestInsert_PositionBeyondLengthGoesToTheBack(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := newInsertSvc(env, oneUser(specCustomerB)).InsertTicket(ctx, env.queueID, specOwnerID, "b@test.com", 10)

	require.NoError(t, err)
	assert.Equal(t, []int64{specCustomerA, specCustomerB}, waitingUserIDs(env))
}

func TestInsert_EmptyQueueAnyPositionIsFirst(t *testing.T) {
	env := newSpecEnv(t)

	got, err := newInsertSvc(env, oneUser(specCustomerA)).InsertTicket(context.Background(), env.queueID, specOwnerID, "a@test.com", 3)

	require.NoError(t, err)
	assert.Equal(t, 1, got.TicketNumber)
	assert.Equal(t, []int64{specCustomerA}, waitingUserIDs(env))
}

func TestInsert_SequentialInsertsAtSamePositionKeepBothAndOrder(t *testing.T) {
	// Perilaku sekuensial: sisip terakhir menempati posisi yang diminta, sisip sebelumnya bergeser satu ke belakang.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)

	_, err := newInsertSvc(env, oneUser(specCustomerC)).InsertTicket(ctx, env.queueID, specOwnerID, "c@test.com", 2)
	require.NoError(t, err)
	_, err = newInsertSvc(env, oneUser(specCustomerD)).InsertTicket(ctx, env.queueID, specOwnerID, "d@test.com", 2)
	require.NoError(t, err)

	assert.Equal(t, []int64{specCustomerA, specCustomerD, specCustomerC, specCustomerB}, waitingUserIDs(env))
}

func TestInsert_ExhaustedSpaceRebalancesAndStillInserts(t *testing.T) {
	// Sort_key dua tiket berdampingan (satu unit) tidak menyisakan titik tengah: harus dinomori ulang.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	for _, tk := range env.tickets.tickets {
		if tk.UserID == specCustomerA {
			tk.SortKey = 100
		}
		if tk.UserID == specCustomerB {
			tk.SortKey = 101
		}
	}

	_, err := newInsertSvc(env, oneUser(specCustomerC)).InsertTicket(ctx, env.queueID, specOwnerID, "c@test.com", 2)

	require.NoError(t, err)
	assert.Equal(t, []int64{specCustomerA, specCustomerC, specCustomerB}, waitingUserIDs(env))
	assert.GreaterOrEqual(t, env.tickets.rebalances, 1)
}

func TestInsert_ErrorCases(t *testing.T) {
	ctx := context.Background()

	t.Run("akun tidak ditemukan", func(t *testing.T) {
		env := newSpecEnv(t)
		_, err := newInsertSvc(env, fakeAccounts{}).InsertTicket(ctx, env.queueID, specOwnerID, "x", 1)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("username ganda ditolak", func(t *testing.T) {
		env := newSpecEnv(t)
		two := fakeAccounts{users: []model.User{{ID: specCustomerA}, {ID: specCustomerB}}}
		_, err := newInsertSvc(env, two).InsertTicket(ctx, env.queueID, specOwnerID, "budi", 1)
		assert.ErrorIs(t, err, apperror.ErrAmbiguousAccount)
		assert.Empty(t, waitingUserIDs(env), "tidak ada tiket yang dibuat")
	})

	t.Run("akun sudah punya tiket aktif", func(t *testing.T) {
		env := newSpecEnv(t)
		_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
		_, err := newInsertSvc(env, oneUser(specCustomerA)).InsertTicket(ctx, env.queueID, specOwnerID, "a", 1)
		assert.ErrorIs(t, err, apperror.ErrAlreadyJoined)
	})

	t.Run("posisi kurang dari 1", func(t *testing.T) {
		env := newSpecEnv(t)
		_, err := newInsertSvc(env, oneUser(specCustomerA)).InsertTicket(ctx, env.queueID, specOwnerID, "a", 0)
		assert.ErrorIs(t, err, apperror.ErrInvalidPosition)
	})

	t.Run("non-owner ditolak", func(t *testing.T) {
		env := newSpecEnv(t)
		_, err := newInsertSvc(env, oneUser(specCustomerB)).InsertTicket(ctx, env.queueID, specCustomerA, "b", 1)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("antrian ditutup", func(t *testing.T) {
		env := newSpecEnv(t)
		require.NoError(t, env.queues.CloseQueue(ctx, env.queueID))
		_, err := newInsertSvc(env, oneUser(specCustomerB)).InsertTicket(ctx, env.queueID, specOwnerID, "b", 1)
		assert.ErrorIs(t, err, apperror.ErrQueueClosed)
	})

	t.Run("tanpa account finder gagal tertutup", func(t *testing.T) {
		env := newSpecEnv(t)
		svc := service.NewTicketService(env.tickets, fakeTxRunner{}, env.events).WithQueueGuard(env.qs)
		_, err := svc.InsertTicket(ctx, env.queueID, specOwnerID, "b", 1)
		assert.Error(t, err)
	})
}

func TestInsert_AccountWhoAlreadyCompletedGetsNewNumber(t *testing.T) {
	// Akun yang tiket lamanya sudah selesai boleh disisipkan lagi, dengan nomor baru.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil) // A selesai

	got, err := newInsertSvc(env, oneUser(specCustomerA)).InsertTicket(ctx, env.queueID, specOwnerID, "a", 1)

	require.NoError(t, err)
	assert.Equal(t, 3, got.TicketNumber, "nomor baru, bukan nomor lama")
	assert.Equal(t, []int64{specCustomerA}, waitingUserIDs(env))
}
