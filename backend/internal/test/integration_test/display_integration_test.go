//go:build integration

package integration_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/database"
	"qms/internal/repository"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisplayRepo_ConfigLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewDisplayRepository(db)
	owner := seedOwner(t, db, "o@x.test", time.Now().Add(time.Hour))
	q := createQueue(t, db, owner, "Loket")

	_, err := repo.Get(ctx, q.ID)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	_, err = repo.Rotate(ctx, q.ID, "x")
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	d, err := repo.Upsert(ctx, q.ID, "token-1", 5, 0)
	require.NoError(t, err)
	assert.Equal(t, "token-1", d.Token)

	again, err := repo.Upsert(ctx, q.ID, "ignored-token", 3, 2)
	require.NoError(t, err)
	assert.Equal(t, "token-1", again.Token, "upsert mengubah jumlah, bukan token")
	assert.Equal(t, 3, again.UpcomingCount)
	assert.Equal(t, 2, again.SkippedCount)

	byToken, err := repo.GetByToken(ctx, "token-1")
	require.NoError(t, err)
	assert.Equal(t, q.ID, byToken.QueueID)

	rot, err := repo.Rotate(ctx, q.ID, "token-2")
	require.NoError(t, err)
	assert.Equal(t, "token-2", rot.Token)
	assert.True(t, rot.RotatedAt.Valid)
	_, err = repo.GetByToken(ctx, "token-1")
	assert.ErrorIs(t, err, apperror.ErrNotFound, "token lama tidak lagi berlaku")

	require.NoError(t, repo.Delete(ctx, q.ID))
	require.NoError(t, repo.Delete(ctx, q.ID), "idempoten")
	_, err = repo.GetByToken(ctx, "token-2")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestDisplayRepo_ConstraintsRejectOutOfRangeAndDuplicateToken(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewDisplayRepository(db)
	owner := seedOwner(t, db, "o@x.test", time.Now().Add(time.Hour))
	q1 := createQueue(t, db, owner, "A")
	q2 := createQueue(t, db, owner, "B")

	_, err := repo.Upsert(ctx, q1.ID, "t1", 11, 0)
	assert.Error(t, err, "CHECK upcoming 0..10")
	_, err = repo.Upsert(ctx, q1.ID, "t1", 5, -1)
	assert.Error(t, err, "CHECK skipped 0..10")

	_, err = repo.Upsert(ctx, q1.ID, "same", 5, 0)
	require.NoError(t, err)
	_, err = repo.Upsert(ctx, q2.ID, "same", 5, 0)
	assert.Error(t, err, "token unik antar antrian")
}

func TestDisplayRepo_DeletingQueueRemovesDisplay(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewDisplayRepository(db)
	owner := seedOwner(t, db, "o@x.test", time.Now().Add(time.Hour))
	q := createQueue(t, db, owner, "A")
	_, err := repo.Upsert(ctx, q.ID, "t", 5, 0)
	require.NoError(t, err)

	_, err = db.Exec(`DELETE FROM queues WHERE id = $1`, q.ID)
	require.NoError(t, err)
	_, err = repo.GetByToken(ctx, "t")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestDisplayRepo_SnapshotQueries(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewDisplayRepository(db)
	tickets := repository.NewTicketRepository(db)
	counters := repository.NewCounterRepository(db)
	owner := seedOwner(t, db, "o@x.test", time.Now().Add(time.Hour))
	q := createQueue(t, db, owner, "Loket")

	var users []int64
	for i := 0; i < 6; i++ {
		users = append(users, seedUser(t, db, "u"+string(rune('a'+i))+"@x.test"))
	}
	var tk []int64
	for _, u := range users {
		tk = append(tk, joinTicket(t, db, q.ID, u).ID)
	}
	var defaultCounter int64
	require.NoError(t, db.Get(&defaultCounter, `SELECT id FROM counters WHERE queue_id = $1 AND is_default`, q.ID))

	// Tiket 1 dilayani di counter default, tiket 2 dilewati; sisanya menunggu.
	require.NoError(t, database.WithTx(ctx, db, func(tx *sqlx.Tx) error {
		// Satu counter hanya boleh melayani satu tiket: tiket 2 dilayani lalu dilewati, baru tiket 1 dilayani.
		if err := tickets.MarkServing(ctx, tx, tk[1], defaultCounter); err != nil {
			return err
		}
		if err := tickets.MarkSkipped(ctx, tx, tk[1]); err != nil {
			return err
		}
		return tickets.MarkServing(ctx, tx, tk[0], defaultCounter)
	}))

	info, err := repo.QueueInfo(ctx, q.ID)
	require.NoError(t, err)
	assert.Equal(t, "Loket", info.Name)
	assert.Equal(t, "active", info.Status)

	up, err := repo.UpcomingNumbers(ctx, q.ID, 3)
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 4, 5}, up, "urut antrian, dibatasi LIMIT")
	none, err := repo.UpcomingNumbers(ctx, q.ID, 0)
	require.NoError(t, err)
	assert.Empty(t, none)

	sk, err := repo.SkippedNumbers(ctx, q.ID, 5)
	require.NoError(t, err)
	assert.Equal(t, []int64{2}, sk)

	list, err := counters.ListWithServing(ctx, q.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, int64(1), list[0].ServingNumber.Int64)
}
