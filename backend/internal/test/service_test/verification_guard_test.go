package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/service"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pengiriman email dan penandaan terverifikasi dilakukan auth-service. Backend hanya membaca status verifikasi dan
// menolak join dan buat antrian bila belum terverifikasi; tes di bawah menjaga perilaku itu.

// fakeVerificationStore: status verifikasi per user di memori. Nil/absen = user tidak ada.
type fakeVerificationStore struct {
	verified map[int64]bool
}

func (s *fakeVerificationStore) IsVerified(_ context.Context, userID int64) (bool, error) {
	v, ok := s.verified[userID]
	if !ok {
		return false, apperror.ErrNotFound
	}
	return v, nil
}

func TestRequireVerified_ReflectsAccountState(t *testing.T) {
	guard := service.NewEmailVerificationGuard(&fakeVerificationStore{verified: map[int64]bool{7: false, 8: true}})
	ctx := context.Background()

	assert.ErrorIs(t, guard.RequireVerified(ctx, 7), apperror.ErrEmailNotVerified)
	assert.NoError(t, guard.RequireVerified(ctx, 8))
	assert.ErrorIs(t, guard.RequireVerified(ctx, 999), apperror.ErrNotFound)
}

func TestRequireVerified_ReadsLiveStatusNotCachedAtLogin(t *testing.T) {
	// Status dibaca dari database pada tiap panggilan, jadi verifikasi berlaku tanpa login ulang.
	store := &fakeVerificationStore{verified: map[int64]bool{7: false}}
	guard := service.NewEmailVerificationGuard(store)
	ctx := context.Background()

	assert.ErrorIs(t, guard.RequireVerified(ctx, 7), apperror.ErrEmailNotVerified)
	store.verified[7] = true
	assert.NoError(t, guard.RequireVerified(ctx, 7))
}

// --- Guard di join dan buat antrian ---

type fakeGuard struct {
	mu       sync.Mutex
	verified map[int64]bool
}

func (g *fakeGuard) RequireVerified(_ context.Context, userID int64) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.verified[userID] {
		return apperror.ErrEmailNotVerified
	}
	return nil
}

func (g *fakeGuard) set(userID int64, v bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.verified[userID] = v
}

func TestJoin_UnverifiedUserIsRejectedWithoutTicket(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{}}
	env.ts.WithVerificationGuard(guard)

	_, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	assert.ErrorIs(t, err, apperror.ErrEmailNotVerified)
	list, err := env.ts.ListTickets(context.Background(), env.queueID, env.ownerID, nil)
	require.NoError(t, err)
	assert.Empty(t, list, "tidak ada tiket yang dibuat")
	assert.Empty(t, env.events.snapshot(), "tidak ada event yang disiarkan")
}

func TestJoin_VerifiedUserJoinsNormally(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{specCustomerA: true}}
	env.ts.WithVerificationGuard(guard)

	ticket, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	require.NoError(t, err)
	assert.Equal(t, 1, ticket.TicketNumber)
}

func TestJoin_RejoinReturnsExistingTicketEvenIfNoLongerVerified(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{specCustomerA: true}}
	env.ts.WithVerificationGuard(guard)
	first, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)
	require.NoError(t, err)
	guard.set(specCustomerA, false)

	again, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	require.NoError(t, err, "scan ulang tidak berubah menjadi galat")
	assert.Equal(t, first.ID, again.ID)
}

func TestJoin_NoGuardMeansNoCheck(t *testing.T) {
	env := newSpecEnv(t)

	_, err := env.ts.JoinQueue(context.Background(), env.queueID, specCustomerA)

	assert.NoError(t, err)
}

func TestCreateQueue_UnverifiedOwnerIsRejectedBeforeSubscriptionCheck(t *testing.T) {
	env := newSpecEnv(t)
	guard := &fakeGuard{verified: map[int64]bool{}}
	env.qs.WithVerificationGuard(guard)
	// pemilik tanpa langganan sekalipun mendapat ErrEmailNotVerified lebih dulu, bukan ErrSubscriptionInactive
	const noSubscription int64 = 555

	_, err := env.qs.CreateQueue(context.Background(), noSubscription, "Loket", "")
	assert.ErrorIs(t, err, apperror.ErrEmailNotVerified)

	_, err = env.qs.CreateQueue(context.Background(), env.ownerID, "Loket 2", "")
	assert.ErrorIs(t, err, apperror.ErrEmailNotVerified)

	guard.set(env.ownerID, true)
	_, err = env.qs.CreateQueue(context.Background(), env.ownerID, "Loket 2", "")
	assert.NoError(t, err)

	guard.set(noSubscription, true)
	_, err = env.qs.CreateQueue(context.Background(), noSubscription, "Loket", "")
	assert.ErrorIs(t, err, apperror.ErrSubscriptionInactive, "setelah terverifikasi, aturan langganan berlaku seperti biasa")
}
