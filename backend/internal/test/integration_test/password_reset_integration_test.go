//go:build integration

package integration_test

import (
	"context"
	"qms/internal/apperror"
	"qms/internal/repository"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPasswordResetRepo_FullCycle(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewPasswordResetRepository(db)
	uid := seedUser(t, db, "reset@x.test")

	u, err := repo.FindForReset(ctx, "reset@x.test")
	require.NoError(t, err)
	assert.Equal(t, uid, u.ID)
	_, err = repo.FindForReset(ctx, "nope@x.test")
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	changed, err := repo.PasswordChangedAt(ctx, uid)
	require.NoError(t, err)
	assert.Nil(t, changed, "belum pernah diganti")

	require.NoError(t, repo.CreateToken(ctx, uid, "password_reset", "hash-1", time.Now().Add(time.Hour), "1.2.3.4"))
	n, err := repo.CountRecentTokens(ctx, uid, "password_reset", time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// Token baru membatalkan yang lama.
	require.NoError(t, repo.CreateToken(ctx, uid, "password_reset", "hash-2", time.Now().Add(time.Hour), ""))
	assert.ErrorIs(t, repo.ResetPassword(ctx, "hash-1", "newhash"), apperror.ErrInvalidResetToken)

	require.NoError(t, repo.ResetPassword(ctx, "hash-2", "newhash"))
	var pw string
	require.NoError(t, db.Get(&pw, `SELECT password_hash FROM users WHERE id = $1`, uid))
	assert.Equal(t, "newhash", pw)
	changed, err = repo.PasswordChangedAt(ctx, uid)
	require.NoError(t, err)
	require.NotNil(t, changed)
	assert.WithinDuration(t, time.Now(), *changed, 10*time.Second)

	assert.ErrorIs(t, repo.ResetPassword(ctx, "hash-2", "other"), apperror.ErrInvalidResetToken, "sekali pakai")
	require.NoError(t, db.Get(&pw, `SELECT password_hash FROM users WHERE id = $1`, uid))
	assert.Equal(t, "newhash", pw, "percobaan kedua tidak mengubah password")

	_, err = repo.PasswordChangedAt(ctx, 999999)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestPasswordResetRepo_ExpiredTokenRejectedAndPasswordUntouched(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewPasswordResetRepository(db)
	uid := seedUser(t, db, "exp@x.test")
	require.NoError(t, repo.CreateToken(ctx, uid, "password_reset", "old", time.Now().Add(-time.Minute), ""))

	assert.ErrorIs(t, repo.ResetPassword(ctx, "old", "newhash"), apperror.ErrInvalidResetToken)
	var pw string
	require.NoError(t, db.Get(&pw, `SELECT password_hash FROM users WHERE id = $1`, uid))
	assert.Equal(t, "h", pw)
}

func TestPasswordResetRepo_ConcurrentUseOfSameTokenSucceedsOnce(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewPasswordResetRepository(db)
	uid := seedUser(t, db, "race@x.test")
	require.NoError(t, repo.CreateToken(ctx, uid, "password_reset", "race", time.Now().Add(time.Hour), ""))

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if repo.ResetPassword(ctx, "race", "newhash") == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, ok)
}

func TestPasswordResetRepo_TokensCascadeWithUser(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	repo := repository.NewPasswordResetRepository(db)
	uid := seedUser(t, db, "gone@x.test")
	require.NoError(t, repo.CreateToken(ctx, uid, "password_reset", "tok", time.Now().Add(time.Hour), ""))

	_, err := db.Exec(`DELETE FROM users WHERE id = $1`, uid)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.Get(&n, `SELECT count(*) FROM user_action_tokens`))
	assert.Zero(t, n)
}
