package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Serving per counter (langkah 4): tiap counter melayani nomornya sendiri; aksi pada satu counter tidak
// menyentuh nomor counter lain. Pengaman expected_ticket_id (D9) menolak aksi dari perangkat yang tertinggal.

// newTwoCounterEnv: antrian dengan counter default (1) dan Loket 2 (ID 2).
func newTwoCounterEnv(t *testing.T) *specEnv {
	env := newSpecEnv(t)
	env.tickets.counters = map[int64]model.Counter{
		2: {ID: 2, QueueID: env.queueID, Name: "Loket 2"},
	}
	return env
}

func ptr(v int64) *int64 { return &v }

func TestMultiCounter_NextOnOneCounterKeepsTheOtherServing(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	b, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)

	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	servedB, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 2, nil)
	require.NoError(t, err)

	assert.Equal(t, b.ID, servedB.ID, "counter 2 menerima waiting teratas")
	assert.Equal(t, int64(2), servedB.CalledCounterID)
	assert.Equal(t, "Loket 2", servedB.CounterName)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	require.Len(t, st.Serving, 2, "dua counter melayani bersamaan")
	assert.Equal(t, 2, st.Serving[0].TicketNumber, "yang paling baru dipanggil dulu")
	assert.Equal(t, 1, st.Serving[1].TicketNumber, "nomor counter 1 tidak terganggu")
	assert.Equal(t, 0, env.tickets.completes, "Next di counter 2 tidak menurunkan nomor counter 1")
}

func TestMultiCounter_ExpectedTicketMatchesAdvances(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	a, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	// Perangkat melihat A sebagai nomor counter 1 yang sedang dilayani: boleh maju.
	got, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, ptr(a.ID))

	require.NoError(t, err)
	assert.Equal(t, specCustomerB, got.UserID, "B sekarang dilayani")
}

func TestMultiCounter_StaleExpectedTicketIsRejectedWithoutChange(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	a, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	completesBefore := env.tickets.completes

	// Perangkat tertinggal mengira counter 1 kosong (0), padahal A sedang dilayani.
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, ptr(0))

	assert.ErrorIs(t, err, apperror.ErrCounterStateChanged)
	assert.Equal(t, completesBefore, env.tickets.completes, "tidak ada perubahan data")
	st, err := env.ts.QueueStatus(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	require.NotNil(t, st.CurrentNumber)
	assert.Equal(t, a.TicketNumber, *st.CurrentNumber)
}

func TestMultiCounter_EmptyCounterExpectingZeroAdvances(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	got, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 2, ptr(0))

	require.NoError(t, err)
	assert.Equal(t, int64(2), got.CalledCounterID)
}

func TestMultiCounter_SkipOnOneCounterDoesNotTouchTheOther(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	a, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	b, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerD)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil) // A di counter 1
	require.NoError(t, err)
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 2, nil) // B di counter 2
	require.NoError(t, err)

	called, err := env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 2, nil)

	require.NoError(t, err)
	assert.Equal(t, int64(2), called.CalledCounterID, "pengganti B ikut counter 2")
	st, err := env.ts.QueueStatus(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	require.Len(t, st.Serving, 2)
	assert.Equal(t, a.TicketNumber, st.Serving[1].TicketNumber, "A di counter 1 tetap dilayani")
	_ = b
}

func TestMultiCounter_SkipWithStaleExpectedIsRejected(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	_, err = env.ts.SkipTicket(ctx, env.queueID, specOwnerID, 0, ptr(999))

	assert.ErrorIs(t, err, apperror.ErrCounterStateChanged)
}

func TestMultiCounter_RecallOnlyTouchesTheRequestedCounter(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	a, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	b, _ := env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 2, nil)
	require.NoError(t, err)
	env.tickets.backdateCall(a.ID, 5*time.Second)
	env.tickets.backdateCall(b.ID, 5*time.Second)

	got, err := env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 2, nil)

	require.NoError(t, err)
	assert.Equal(t, b.ID, got.ID, "yang dipanggil ulang adalah nomor counter 2")
	_, err = env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 2, nil)
	assert.ErrorIs(t, err, apperror.ErrAlreadyRecalled)
	// Counter 1 belum pernah dipanggil ulang: masih boleh.
	gotA, err := env.ts.RecallTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, a.ID, gotA.ID)
}

func TestMultiCounter_CancelPromotesWaitingIntoSameCounter(t *testing.T) {
	env := newTwoCounterEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerB)
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerC)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)
	_, err = env.ts.NextTicket(ctx, env.queueID, specOwnerID, 2, nil)
	require.NoError(t, err)

	_, err = env.ts.CancelTicket(ctx, env.queueID, specCustomerB)

	require.NoError(t, err)
	st, err := env.ts.QueueStatus(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	require.Len(t, st.Serving, 2, "counter 2 tetap melayani, kini dengan C")
	assert.Equal(t, int64(2), st.Serving[0].CounterID)
	assert.Equal(t, 3, st.Serving[0].TicketNumber)
	assert.Equal(t, 1, st.Serving[1].TicketNumber, "counter 1 tidak berubah")
}

func TestMultiCounter_SingleCounterQueueStatusHidesDefaultName(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0, nil)
	require.NoError(t, err)

	st, err := env.ts.QueueStatus(ctx, env.queueID, specOwnerID)

	require.NoError(t, err)
	require.Len(t, st.Serving, 1)
	assert.Nil(t, st.Serving[0].CounterName, "counter default tidak menampilkan nama")
	assert.Nil(t, st.CurrentCounterName)
}
