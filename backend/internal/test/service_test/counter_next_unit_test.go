package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Next memakai counter yang diminta (counter_id), bukan selalu counter default. Counter milik antrian lain ditolak.

func TestNext_UsesRequestedCounterAndItsName(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	env.tickets.counters = map[int64]model.Counter{
		2: {ID: 2, QueueID: env.queueID, Name: "Loket 2"},
	}
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	got, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 2)

	require.NoError(t, err)
	assert.Equal(t, int64(2), got.CalledCounterID, "nomor dicatat ke counter yang memanggil")
	assert.Equal(t, "Loket 2", got.CounterName, "nama counter yang dipilih, bukan counter default")
}

func TestNext_CounterFromAnotherQueueIsNotFoundAndChangesNothing(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	env.tickets.counters = map[int64]model.Counter{
		3: {ID: 3, QueueID: env.queueID + 999, Name: "Loket Asing"},
	}
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 3)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
	assert.Equal(t, 0, env.tickets.completes, "tidak ada perubahan status")
	assert.Equal(t, 1, len(env.tickets.waitingInOrder(env.queueID)), "nomor tetap menunggu")
}

func TestNext_UnknownCounterIsNotFound(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	_, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 999)

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestNext_CounterZeroIsDefaultAndHidesName(t *testing.T) {
	env := newSpecEnv(t)
	ctx := context.Background()
	_, _ = env.ts.JoinQueue(ctx, env.queueID, specCustomerA)

	got, err := env.ts.NextTicket(ctx, env.queueID, specOwnerID, 0)

	require.NoError(t, err)
	assert.Equal(t, int64(1), got.CalledCounterID, "counter 0 = counter default")
	assert.Empty(t, got.CounterName, "nama counter default disembunyikan")
}
