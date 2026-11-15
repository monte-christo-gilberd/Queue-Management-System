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

// Customer cancel (4.9): nomor dibatalkan, tidak dihapus; nomor yang sedang dilayani membuat berikutnya naik.

func TestCancel_WaitingTicketLeavesOrderAndKeepsOthers(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)

	got, err := env.ts.CancelTicket(ctx, env.queueID, specCustomerA)

	require.NoError(t, err)
	assert.Equal(t, "cancelled", got.Status)
	assert.True(t, got.CancelledAt.Valid)
	waiting, err := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"waiting"})
	require.NoError(t, err)
	require.Len(t, waiting, 1)
	assert.Equal(t, specCustomerB, waiting[0].UserID)
	assert.Equal(t, 1, env.events.count(service.EventTicketCancelled))
	assert.Equal(t, 0, env.events.count(service.EventTicketNext))
}

func TestCancel_ServingTicketAdvancesNextWaiting(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = env.ts.CancelTicket(ctx, env.queueID, specCustomerA)

	require.NoError(t, err)
	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerB)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber, "waiting teratas naik ke currently_served")
	assert.Equal(t, 1, env.events.count(service.EventTicketCancelled))
	assert.Equal(t, 2, env.events.count(service.EventTicketNext), "Next pertama dan kenaikan setelah cancel")
}

func TestCancel_ServingTicketWithNobodyWaitingIsNotAnError(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = env.ts.CancelTicket(ctx, env.queueID, specCustomerA)

	require.NoError(t, err)
	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.Nil(t, st.CurrentNumber, "tidak ada yang menunggu, tidak ada yang dilayani")
}

func TestCancel_TwiceIsNoOpWithoutSecondEvent(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.CancelTicket(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)

	got, err := env.ts.CancelTicket(ctx, env.queueID, specCustomerA)

	require.NoError(t, err)
	assert.Equal(t, "cancelled", got.Status)
	assert.Equal(t, 1, env.events.count(service.EventTicketCancelled))
}

func TestCancel_CompletedTicketIsNoOp(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil) // A selesai

	got, err := env.ts.CancelTicket(ctx, env.queueID, specCustomerA)

	require.NoError(t, err)
	assert.Equal(t, "completed", got.Status)
	assert.Equal(t, 0, env.events.count(service.EventTicketCancelled))
}

func TestCancel_UserWithoutTicketIsNotFound(t *testing.T) {
	env := newSpecEnv(t)

	_, err := env.ts.CancelTicket(context.Background(), env.queueID, specCustomerD)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestCancel_ClosedQueueIsRejected(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, env.queues.CloseQueue(ctx, env.queueID))

	_, err := env.ts.CancelTicket(ctx, env.queueID, specCustomerA)

	assert.ErrorIs(t, err, apperror.ErrQueueClosed)
}

func TestCancel_RejoinAfterCancelGetsNewNumber(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.CancelTicket(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)

	rejoined, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	require.NoError(t, err)
	assert.Equal(t, 2, rejoined.TicketNumber)
	assert.Equal(t, "waiting", rejoined.Status)
}

func TestCancel_WithoutGuardFailsClosed(t *testing.T) {
	env := newSpecEnv(t)
	svc := service.NewTicketService(env.tickets, fakeTxRunner{})

	_, err := svc.CancelTicket(context.Background(), env.queueID, specCustomerA)

	assert.Error(t, err)
}

// Owner remove (4.10): nomor berperilaku seperti skip tanpa fallback; status akhir deleted; tidak tampil di skipped.

func TestRemove_WaitingTicketIsNotRequeuedEvenWithBackFallback(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	env.tickets.skipSettings = map[int64]model.SkipSettings{
		env.queueID: {Fallback: model.SkipFallbackBack, MaxReturns: 3},
	}
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	target, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerC)
	require.NoError(t, err)

	got, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, target.ID)

	require.NoError(t, err)
	assert.Equal(t, "deleted", got.Status)
	assert.Equal(t, 0, got.SkipCount, "skip_count tidak naik")
	waiting, err := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"waiting"})
	require.NoError(t, err)
	assert.Len(t, waiting, 2, "nomor yang dihapus tidak kembali ke waiting")
	assert.Equal(t, 1, env.events.count(service.EventTicketRemoved))
}

func TestRemove_ServingTicketAdvancesNextWaiting(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	first, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	got, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, first.ID)

	require.NoError(t, err)
	assert.Equal(t, "deleted", got.Status)
	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerB)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, 2, *st.CurrentNumber)
	assert.Equal(t, 1, env.events.count(service.EventTicketRemoved))
	assert.Equal(t, 2, env.events.count(service.EventTicketNext))
}

func TestRemove_ServingTicketWithNobodyWaiting(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	first, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, first.ID)

	require.NoError(t, err)
	st, err := env.ts.QueueStatus(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.Nil(t, st.CurrentNumber)
}

func TestRemove_AlreadyInactiveIsNoOp(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	first, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, first.ID)
	require.NoError(t, err)

	got, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, first.ID)

	require.NoError(t, err)
	assert.Equal(t, "deleted", got.Status)
	assert.Equal(t, 1, env.events.count(service.EventTicketRemoved), "remove kedua tidak memancarkan event")
}

// useRemoveSkip mengatur antrian ke perilaku skip 'remove' (bawaan sekarang 'back'), supaya tes ini menguji remove.
func useRemoveSkip(t *testing.T, env *specEnv) {
	t.Helper()
	remove := model.SkipSettings{Fallback: model.SkipFallbackRemove, MaxReturns: 1}
	_, err := env.qs.UpdateSkipSettings(context.Background(), env.queueID, specOwnerID, remove)
	require.NoError(t, err)
	// Jalur skip tiket membaca pengaturan dari ticket repo (fake terpisah), jadi isi juga di sana.
	if env.tickets.skipSettings == nil {
		env.tickets.skipSettings = map[int64]model.SkipSettings{}
	}
	env.tickets.skipSettings[env.queueID] = remove
}

func TestRemove_SkippedTicketIsNoOp(t *testing.T) {
	env := newSpecEnv(t)
	useRemoveSkip(t, env)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	second, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	_, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, nil) // A dilewati (fallback remove)
	require.NoError(t, err)

	first, err := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"skipped"})
	require.NoError(t, err)
	require.Len(t, first, 1)
	got, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, first[0].ID)

	require.NoError(t, err)
	assert.Equal(t, "skipped", got.Status)
	assert.Equal(t, 0, env.events.count(service.EventTicketRemoved))
	_ = second
}

func TestRemove_DeletedTicketIsNotShownAsSkipped(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	target, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, target.ID)
	require.NoError(t, err)

	skipped, err := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"skipped"})
	require.NoError(t, err)
	assert.Empty(t, skipped, "deleted tidak tampil di daftar skipped")
	all, err := env.ts.ListTickets(ctx, env.queueID, specOwnerID, []string{"deleted"})
	require.NoError(t, err)
	assert.Len(t, all, 1)
}

func TestRemove_NonOwnerIsNotFound(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	target, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := env.ts.RemoveTicket(ctx, env.queueID, specCustomerA, target.ID)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestRemove_ClosedQueueIsRejected(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	target, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, env.queues.CloseQueue(ctx, env.queueID))

	_, err := env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, target.ID)

	assert.ErrorIs(t, err, apperror.ErrQueueClosed)
}

func TestRemove_TicketFromAnotherQueueIsNotFound(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	otherQueue, err := env.qs.CreateQueue(ctx, specOwnerID, "Loket Dua", "")
	require.NoError(t, err)
	foreign, _ := env.ts.JoinQueue(ctx, otherQueue.ID, specCustomerA)

	_, err = env.ts.RemoveTicket(ctx, env.queueID, specOwnerID, foreign.ID)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

// Regresi: Next dan Skip tetap berfungsi setelah urutan kunci Next diubah.
func TestNext_LockOrderChangeKeepsNextAndSkipWorking(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)

	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	assert.ErrorIs(t, err, apperror.ErrQueueEmpty)
}
