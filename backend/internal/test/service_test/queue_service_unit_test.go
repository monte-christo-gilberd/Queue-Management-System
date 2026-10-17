package service_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"qms/internal/service"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const specOtherOwnerID int64 = 2

func TestSpec_CreateQueue_InactiveSubscriptionBlocksCreation(t *testing.T) {
	// US-21 AC1 / TC-04-01: owner tanpa langganan aktif diblokir; antrian tidak dibuat.
	env := newSpecEnv(t)
	env.subs.active = map[int64]bool{}
	before := len(env.queues.order)

	_, _, err := env.qs.CreateQueue(context.Background(), specOtherOwnerID, "Antrian Baru", "")
	assert.ErrorIs(t, err, apperror.ErrSubscriptionInactive)
	assert.Len(t, env.queues.order, before)
}

func TestSpec_CreateQueue_ActiveSubscriptionCreatesEmptyActiveQueue(t *testing.T) {
	// US-1 / 4.1 langkah 2: antrian baru dibuat kosong dan aktif, milik owner yang membuatnya.
	env := newSpecEnv(t)
	ctx := context.Background()

	q, authz, err := env.qs.CreateQueue(ctx, specOwnerID, "Kasir 2", "Pembayaran")
	require.NoError(t, err)
	assert.Equal(t, "active", q.Status)
	assert.Equal(t, "Kasir 2", q.QueueName)
	assert.Equal(t, specOwnerID, q.OwnerID)
	assert.Equal(t, q.ID, authz.QueueID)
	assert.Empty(t, env.tickets.ticketsOf(q.ID))
}

func TestSpec_CloseQueue_OwnerCloseWorksWithoutActiveSubscription(t *testing.T) {
	// Change log 2026-10-02 / 4.18: menutup antrian tidak memerlukan langganan aktif;
	// antrian yang sudah ada tetap bisa dikelola.
	env := newSpecEnv(t)
	env.subs.active = map[int64]bool{}
	ctx := context.Background()

	require.NoError(t, env.qs.CloseQueue(ctx, env.queueID, specOwnerID))

	q, err := env.qs.GetActiveQueue(ctx, env.queueID)
	assert.ErrorIs(t, err, apperror.ErrQueueClosed)
	assert.Equal(t, model.Queue{}, q)
	assert.Equal(t, 1, env.events.count(service.EventQueueClosed))
}

func TestSpec_CloseQueue_NonOwnerIsNotFoundAndQueueStaysActive(t *testing.T) {
	// TC-08-03 / 4.1: owner lain tidak bisa menutup antrian; antrian tetap aktif.
	env := newSpecEnv(t)
	ctx := context.Background()

	err := env.qs.CloseQueue(ctx, env.queueID, specOtherOwnerID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	_, err = env.qs.GetActiveQueue(ctx, env.queueID)
	assert.NoError(t, err)
	assert.Equal(t, 0, env.events.count(service.EventQueueClosed))
}

func TestSpec_UpdateQueue_OwnerChangesNameAndDescription(t *testing.T) {
	// US-2 AC1 / TC-02-03: owner mengubah nama dan deskripsi; perubahan tersimpan dan disiarkan.
	env := newSpecEnv(t)
	ctx := context.Background()

	require.NoError(t, env.qs.UpdateQueue(ctx, env.queueID, specOwnerID, "Loket Baru", "Jam buka 08.00"))

	q, _, err := env.qs.GetQueue(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	assert.Equal(t, "Loket Baru", q.QueueName)
	assert.Equal(t, "Jam buka 08.00", q.QueueDesc.String)
	assert.Equal(t, 1, env.events.count(service.EventQueueUpdated))
}

func TestSpec_UpdateQueue_RequiresActiveSubscription(t *testing.T) {
	// Change log 2026-10-02 / 4.18: mengedit antrian termasuk aksi yang butuh langganan aktif.
	env := newSpecEnv(t)
	env.subs.active = map[int64]bool{}
	ctx := context.Background()

	err := env.qs.UpdateQueue(ctx, env.queueID, specOwnerID, "Diubah", "")
	assert.ErrorIs(t, err, apperror.ErrSubscriptionInactive)

	q, _, err := env.qs.GetQueue(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	assert.Equal(t, "Loket Utama", q.QueueName)
	assert.Equal(t, 0, env.events.count(service.EventQueueUpdated))
}

func TestSpec_UpdateQueue_NonOwnerCannotEdit(t *testing.T) {
	// TC-08-03: owner lain tidak bisa mengubah antrian.
	env := newSpecEnv(t)
	env.subs.active[specOtherOwnerID] = true // owner lain punya langganan aktif, jadi yang diuji hanya kepemilikan
	ctx := context.Background()

	err := env.qs.UpdateQueue(ctx, env.queueID, specOtherOwnerID, "Diambil", "")
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	q, _, err := env.qs.GetQueue(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	assert.Equal(t, "Loket Utama", q.QueueName)
}

func TestSpec_UpdateAuthz_SharedAuthoritySetAppliesToQueue(t *testing.T) {
	// US-3 AC2 / 4.6 langkah 2: satu set izin per antrian; perubahan langsung berlaku
	// dan disiarkan sebagai queue_updated.
	env := newSpecEnv(t)
	ctx := context.Background()

	fresh, err := env.qs.UpdateAuthz(ctx, env.queueID, specOwnerID, model.QueueAuthz{
		CanContinueQueue:  true,
		CanNotifyCustomer: true,
		CanManageNotes:    true,
	})
	require.NoError(t, err)
	assert.True(t, fresh.CanContinueQueue)
	assert.False(t, fresh.CanManageQueue)

	_, stored, err := env.qs.GetQueue(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	assert.True(t, stored.CanContinueQueue)
	assert.True(t, stored.CanNotifyCustomer)

	require.Equal(t, 1, env.events.count(service.EventQueueUpdated))
	payload, ok := env.events.snapshot()[0].Payload.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, dto.NewQueueAuthzResponse(fresh), payload["queue_authz"])
}

func TestSpec_UpdateAuthz_AuthorityConfigIsPerQueue(t *testing.T) {
	// 4.6 edge case: setiap antrian punya konfigurasi izin sendiri dan tidak saling memengaruhi.
	env := newSpecEnv(t)
	ctx := context.Background()
	other, _, err := env.qs.CreateQueue(ctx, specOwnerID, "Kasir 2", "")
	require.NoError(t, err)

	_, err = env.qs.UpdateAuthz(ctx, env.queueID, specOwnerID, model.QueueAuthz{CanContinueQueue: true})
	require.NoError(t, err)

	_, otherAuthz, err := env.qs.GetQueue(ctx, other.ID, specOwnerID)
	require.NoError(t, err)
	assert.False(t, otherAuthz.CanContinueQueue)
}

func TestSpec_UpdateAuthz_NonOwnerCannotChangeAuthorities(t *testing.T) {
	// NFR 7 / 4.6: izin ditegakkan di server; non-owner tidak bisa mengubahnya.
	env := newSpecEnv(t)
	env.subs.active[specOtherOwnerID] = true
	ctx := context.Background()

	_, err := env.qs.UpdateAuthz(ctx, env.queueID, specOtherOwnerID, model.QueueAuthz{CanContinueQueue: true})
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	_, stored, err := env.qs.GetQueue(ctx, env.queueID, specOwnerID)
	require.NoError(t, err)
	assert.False(t, stored.CanContinueQueue)
	assert.Equal(t, 0, env.events.count(service.EventQueueUpdated))
}

func TestSpec_ListQueue_OwnerSeesOnlyOwnQueues(t *testing.T) {
	// 4.1: owner melihat antrian miliknya; antrian owner lain tidak ikut tampil.
	env := newSpecEnv(t)
	env.subs.active[specOtherOwnerID] = true
	ctx := context.Background()
	_, _, err := env.qs.CreateQueue(ctx, specOwnerID, "Kasir 2", "")
	require.NoError(t, err)
	_, _, err = env.qs.CreateQueue(ctx, specOtherOwnerID, "Milik Orang Lain", "")
	require.NoError(t, err)

	queues, err := env.qs.ListQueue(ctx, specOwnerID)
	require.NoError(t, err)
	require.Len(t, queues, 2)
	for _, q := range queues {
		assert.Equal(t, specOwnerID, q.OwnerID)
	}
}

func TestSpec_ListQueue_ViewingDoesNotRequireSubscription(t *testing.T) {
	// Change log 2026-10-02 / TC-04-02: owner tanpa langganan tetap bisa membuka daftar antrian.
	env := newSpecEnv(t)
	env.subs.active = map[int64]bool{}

	queues, err := env.qs.ListQueue(context.Background(), specOwnerID)
	require.NoError(t, err)
	assert.Len(t, queues, 1)
}

func TestSpec_ViewQueue_TicketHolderSeesQueueWithoutAuthorities(t *testing.T) {
	// 4.4 / TC-08-03: pemegang tiket melihat antrian tanpa izin pengelolaan.
	env := newSpecEnv(t)
	ctx := context.Background()
	_, err := env.ts.JoinQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)

	q, authz, isOwner, err := env.qs.ViewQueue(ctx, env.queueID, specCustomerA)
	require.NoError(t, err)
	assert.False(t, isOwner)
	assert.Equal(t, env.queueID, q.ID)
	assert.Equal(t, model.QueueAuthz{}, authz)
}
