package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/repository"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	return sqlxDB, mock
}

const (
	createQueryPattern      = `^INSERT INTO users\(\s*email,\s*username,\s*password_hash\s*\)\s*VALUES\s*\(\$1,\s*\$2,\s*\$3\)\s*RETURNING\s*id,\s*email,\s*username,\s*password_hash,\s*google_id,\s*created_at\s*$`
	findByEmailQueryPattern = `^SELECT id, email, username, password_hash, google_id, created_at FROM users WHERE email=\$1$`
)

func TestUserRepository_FindByEmail(t *testing.T) {
	t.Run("1. Sukses - Data ditemukan", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := repository.NewUserRepository(db)
		now := time.Now()

		expectedUser := model.User{
			ID:           10,
			Email:        "exist@example.com",
			Username:     "existinguser",
			PasswordHash: sql.NullString{String: "hashed_pass", Valid: true},
			GoogleID:     sql.NullString{String: "google-uid-123", Valid: true},
			CreatedAt:    now,
		}

		rows := sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "google_id", "created_at"}).
			AddRow(expectedUser.ID, expectedUser.Email, expectedUser.Username, expectedUser.PasswordHash, expectedUser.GoogleID, expectedUser.CreatedAt)

		mock.ExpectQuery(findByEmailQueryPattern).
			WithArgs("exist@example.com").
			WillReturnRows(rows)

		user, err := repo.FindByEmail(context.Background(), "exist@example.com")

		assert.NoError(t, err)
		assert.Equal(t, expectedUser, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("2. Error - Record tidak ditemukan (sql.ErrNoRows)", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := repository.NewUserRepository(db)

		mock.ExpectQuery(findByEmailQueryPattern).
			WithArgs("notfound@example.com").
			WillReturnError(sql.ErrNoRows)

		user, err := repo.FindByEmail(context.Background(), "notfound@example.com")

		assert.Error(t, err)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
		assert.Equal(t, model.User{}, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("3. Error - PostgreSQL Syntax / Internal Error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := repository.NewUserRepository(db)
		pgErr := &pgconn.PgError{
			Code:    "42703",
			Message: "column does not exist",
		}

		mock.ExpectQuery(findByEmailQueryPattern).
			WithArgs("test@example.com").
			WillReturnError(pgErr)

		user, err := repo.FindByEmail(context.Background(), "test@example.com")

		assert.Error(t, err)
		assert.False(t, errors.Is(err, apperror.ErrNotFound))
		assert.Contains(t, err.Error(), "find user by email:")
		assert.Equal(t, model.User{}, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("4. Error - Database connection failure", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()

		repo := repository.NewUserRepository(db)
		dbErr := errors.New("driver: bad connection")

		mock.ExpectQuery(findByEmailQueryPattern).
			WithArgs("test@example.com").
			WillReturnError(dbErr)

		user, err := repo.FindByEmail(context.Background(), "test@example.com")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "find user by email:")
		assert.ErrorIs(t, err, dbErr)
		assert.Equal(t, model.User{}, user)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
