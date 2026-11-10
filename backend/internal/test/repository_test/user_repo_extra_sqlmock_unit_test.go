package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_FindByID(t *testing.T) {
	cols := []string{"id", "email", "username", "password_hash", "google_id", "created_at", "email_notifications", "email_verified_at"}

	t.Run("found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`FROM users\s+WHERE id = \$1`).
			WithArgs(int64(9)).
			WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(9), "a@b.test", "ani", nil, nil, time.Now(), true, nil))

		got, err := repo.FindByID(context.Background(), 9)
		require.NoError(t, err)
		assert.Equal(t, int64(9), got.ID)
		assert.Equal(t, "ani", got.Username)
	})

	t.Run("not found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`FROM users`).WillReturnError(sql.ErrNoRows)

		_, err := repo.FindByID(context.Background(), 9)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`FROM users`).WillReturnError(errDB)

		_, err := repo.FindByID(context.Background(), 9)
		assert.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "find user by id")
	})
}

func TestUserRepository_SubscriptionExpiresAt(t *testing.T) {
	ts := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("owner", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`SELECT subscription_expires_at FROM owners`).
			WillReturnRows(sqlmock.NewRows([]string{"subscription_expires_at"}).AddRow(ts))

		got, err := repo.SubscriptionExpiresAt(context.Background(), 9)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, ts.Equal(*got))
	})

	t.Run("not an owner", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`SELECT subscription_expires_at FROM owners`).WillReturnError(sql.ErrNoRows)

		got, err := repo.SubscriptionExpiresAt(context.Background(), 9)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`SELECT subscription_expires_at FROM owners`).WillReturnError(errDB)

		_, err := repo.SubscriptionExpiresAt(context.Background(), 9)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUserRepository_UpdateUsername(t *testing.T) {
	cols := []string{"id", "email", "username", "password_hash", "google_id", "created_at"}

	t.Run("updated", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`UPDATE users SET username`).
			WithArgs(int64(9), "baru").
			WillReturnRows(sqlmock.NewRows(cols).AddRow(int64(9), "a@b.test", "baru", nil, nil, time.Now()))

		got, err := repo.UpdateUsername(context.Background(), 9, "baru")
		require.NoError(t, err)
		assert.Equal(t, "baru", got.Username)
	})

	t.Run("not found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`UPDATE users SET username`).WillReturnError(sql.ErrNoRows)

		_, err := repo.UpdateUsername(context.Background(), 9, "baru")
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`UPDATE users SET username`).WillReturnError(errDB)

		_, err := repo.UpdateUsername(context.Background(), 9, "baru")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUserRepository_SetEmailNotifications(t *testing.T) {
	t.Run("updated", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectExec(`UPDATE users SET email_notifications`).
			WithArgs(int64(9), false).
			WillReturnResult(sqlmock.NewResult(0, 1))

		assert.NoError(t, repo.SetEmailNotifications(context.Background(), 9, false))
	})

	t.Run("no such user", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectExec(`UPDATE users SET email_notifications`).WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.SetEmailNotifications(context.Background(), 9, true)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectExec(`UPDATE users SET email_notifications`).WillReturnError(errDB)

		err := repo.SetEmailNotifications(context.Background(), 9, true)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestUserRepository_IsOwner(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM owners`).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		got, err := repo.IsOwner(context.Background(), 9)
		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewUserRepository(db)
		mock.ExpectQuery(`SELECT EXISTS`).WillReturnError(errDB)

		got, err := repo.IsOwner(context.Background(), 9)
		assert.ErrorIs(t, err, errDB)
		assert.False(t, got)
	})
}
