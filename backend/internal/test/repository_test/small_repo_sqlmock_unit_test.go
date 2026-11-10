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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDB = errors.New("db down")

func TestAvatarRepository_PutReturnsUpdatedAt(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()
	repo := repository.NewAvatarRepository(db)
	ts := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO user_avatars`).
		WithArgs(int64(3), []byte("img")).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(ts))

	got, err := repo.Put(context.Background(), 3, []byte("img"))
	require.NoError(t, err)
	assert.True(t, ts.Equal(got))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_PutWrapsError(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()
	repo := repository.NewAvatarRepository(db)

	mock.ExpectQuery(`INSERT INTO user_avatars`).WillReturnError(errDB)

	got, err := repo.Put(context.Background(), 3, []byte("img"))
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "put avatar")
	assert.True(t, got.IsZero())
}

func TestAvatarRepository_Get(t *testing.T) {
	ts := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	t.Run("found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewAvatarRepository(db)
		mock.ExpectQuery(`SELECT data, content_type, updated_at FROM user_avatars`).
			WithArgs(int64(3)).
			WillReturnRows(sqlmock.NewRows([]string{"data", "content_type", "updated_at"}).
				AddRow([]byte("img"), "image/jpeg", ts))

		got, err := repo.Get(context.Background(), 3)
		require.NoError(t, err)
		assert.Equal(t, model.Avatar{Data: []byte("img"), ContentType: "image/jpeg", UpdatedAt: ts}, got)
	})

	t.Run("not found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewAvatarRepository(db)
		mock.ExpectQuery(`SELECT data, content_type`).WillReturnError(sql.ErrNoRows)

		_, err := repo.Get(context.Background(), 3)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewAvatarRepository(db)
		mock.ExpectQuery(`SELECT data, content_type`).WillReturnError(errDB)

		_, err := repo.Get(context.Background(), 3)
		assert.ErrorIs(t, err, errDB)
		assert.Contains(t, err.Error(), "get avatar")
	})
}

func TestAvatarRepository_Delete(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()
	repo := repository.NewAvatarRepository(db)
	mock.ExpectExec(`DELETE FROM user_avatars WHERE user_id = \$1`).
		WithArgs(int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, repo.Delete(context.Background(), 3))
	assert.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectExec(`DELETE FROM user_avatars`).WillReturnError(errDB)
	err := repo.Delete(context.Background(), 3)
	assert.ErrorIs(t, err, errDB)
	assert.Contains(t, err.Error(), "delete avatar")
}

func TestAvatarRepository_UpdatedAt(t *testing.T) {
	ts := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	t.Run("found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewAvatarRepository(db)
		mock.ExpectQuery(`SELECT updated_at FROM user_avatars`).
			WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(ts))

		got, err := repo.UpdatedAt(context.Background(), 3)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, ts.Equal(*got))
	})

	t.Run("no avatar returns nil without error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewAvatarRepository(db)
		mock.ExpectQuery(`SELECT updated_at FROM user_avatars`).WillReturnError(sql.ErrNoRows)

		got, err := repo.UpdatedAt(context.Background(), 3)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewAvatarRepository(db)
		mock.ExpectQuery(`SELECT updated_at FROM user_avatars`).WillReturnError(errDB)

		got, err := repo.UpdatedAt(context.Background(), 3)
		assert.ErrorIs(t, err, errDB)
		assert.Nil(t, got)
	})
}

var noteCols = []string{"ticket_id", "note", "created_at", "updated_at"}

func TestNoteRepository_GetNote(t *testing.T) {
	ts := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

	t.Run("found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`FROM tickets t`).
			WithArgs(int64(5), int64(3)).
			WillReturnRows(sqlmock.NewRows(noteCols).AddRow(int64(5), "catatan", ts, ts))

		got, err := repo.GetNote(context.Background(), 3, 5)
		require.NoError(t, err)
		assert.Equal(t, int64(5), got.TicketID)
		assert.Equal(t, "catatan", got.Note.String)
	})

	t.Run("ticket without note", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`FROM tickets t`).
			WillReturnRows(sqlmock.NewRows(noteCols).AddRow(int64(5), nil, nil, nil))

		got, err := repo.GetNote(context.Background(), 3, 5)
		require.NoError(t, err)
		assert.False(t, got.Note.Valid)
	})

	t.Run("not found", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`FROM tickets t`).WillReturnError(sql.ErrNoRows)

		_, err := repo.GetNote(context.Background(), 3, 5)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`FROM tickets t`).WillReturnError(errDB)

		_, err := repo.GetNote(context.Background(), 3, 5)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestNoteRepository_UpsertNote(t *testing.T) {
	ts := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	upsertCols := []string{"ticket_id", "note", "created_at", "updated_at", "created"}

	t.Run("inserted", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`INSERT INTO ticket_notes`).
			WithArgs(int64(5), int64(3), "baru", int64(9)).
			WillReturnRows(sqlmock.NewRows(upsertCols).AddRow(int64(5), "baru", ts, ts, true))

		got, err := repo.UpsertNote(context.Background(), 3, 5, 9, "baru")
		require.NoError(t, err)
		assert.True(t, got.Created)
		assert.Equal(t, "baru", got.Note.String)
	})

	t.Run("overwritten", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`INSERT INTO ticket_notes`).
			WillReturnRows(sqlmock.NewRows(upsertCols).AddRow(int64(5), "ganti", ts, ts, false))

		got, err := repo.UpsertNote(context.Background(), 3, 5, 9, "ganti")
		require.NoError(t, err)
		assert.False(t, got.Created)
	})

	t.Run("ticket not in queue", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`INSERT INTO ticket_notes`).WillReturnError(sql.ErrNoRows)

		_, err := repo.UpsertNote(context.Background(), 3, 5, 9, "x")
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewNoteRepository(db)
		mock.ExpectQuery(`INSERT INTO ticket_notes`).WillReturnError(errDB)

		_, err := repo.UpsertNote(context.Background(), 3, 5, 9, "x")
		assert.ErrorIs(t, err, errDB)
	})
}

func TestSubscriptionChecker_IsActive(t *testing.T) {
	future := time.Now().Add(72 * time.Hour)
	past := time.Now().Add(-72 * time.Hour)

	cases := []struct {
		name    string
		rowErr  error
		expires *time.Time
		want    bool
		wantErr error
	}{
		{name: "future expiry is active", expires: &future, want: true},
		{name: "past expiry is inactive", expires: &past, want: false},
		{name: "no owner row is inactive", rowErr: sql.ErrNoRows, want: false},
		{name: "db error", rowErr: errDB, want: false, wantErr: errDB},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := setupTestDB(t)
			defer db.Close()
			repo := repository.NewSubscriptionChecker(db)
			q := mock.ExpectQuery(`SELECT subscription_expires_at FROM owners WHERE user_id = \$1`)
			if tc.rowErr != nil {
				q.WillReturnError(tc.rowErr)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"subscription_expires_at"}).AddRow(*tc.expires))
			}

			got, err := repo.IsActive(context.Background(), 9)
			assert.Equal(t, tc.want, got)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPushRepository_UpsertAndDelete(t *testing.T) {
	db, mock := setupTestDB(t)
	defer db.Close()
	repo := repository.NewPushRepository(db)

	mock.ExpectExec(`INSERT INTO push_subscriptions`).
		WithArgs(int64(9), "https://push.test/e", "p", "a").
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.Upsert(context.Background(), 9, "https://push.test/e", "p", "a"))

	mock.ExpectExec(`INSERT INTO push_subscriptions`).WillReturnError(errDB)
	assert.ErrorIs(t, repo.Upsert(context.Background(), 9, "e", "p", "a"), errDB)

	mock.ExpectExec(`DELETE FROM push_subscriptions WHERE user_id = \$1 AND endpoint = \$2`).
		WithArgs(int64(9), "https://push.test/e").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete(context.Background(), 9, "https://push.test/e"))

	mock.ExpectExec(`DELETE FROM push_subscriptions`).WillReturnError(errDB)
	assert.ErrorIs(t, repo.Delete(context.Background(), 9, "e"), errDB)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSessionRepository_PasswordChangedAt(t *testing.T) {
	ts := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	t.Run("changed", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewSessionRepository(db)
		mock.ExpectQuery(`SELECT password_changed_at FROM users WHERE id = \$1`).
			WithArgs(int64(9)).
			WillReturnRows(sqlmock.NewRows([]string{"password_changed_at"}).AddRow(ts))

		got, err := repo.PasswordChangedAt(context.Background(), 9)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.True(t, ts.Equal(*got))
	})

	t.Run("never changed", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewSessionRepository(db)
		mock.ExpectQuery(`SELECT password_changed_at`).
			WillReturnRows(sqlmock.NewRows([]string{"password_changed_at"}).AddRow(nil))

		got, err := repo.PasswordChangedAt(context.Background(), 9)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("user missing", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewSessionRepository(db)
		mock.ExpectQuery(`SELECT password_changed_at`).WillReturnError(sql.ErrNoRows)

		_, err := repo.PasswordChangedAt(context.Background(), 9)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewSessionRepository(db)
		mock.ExpectQuery(`SELECT password_changed_at`).WillReturnError(errDB)

		_, err := repo.PasswordChangedAt(context.Background(), 9)
		assert.ErrorIs(t, err, errDB)
	})
}

func TestVerificationRepository_IsVerified(t *testing.T) {
	t.Run("verified", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewVerificationRepository(db)
		mock.ExpectQuery(`SELECT email_verified_at IS NOT NULL FROM users WHERE id = \$1`).
			WithArgs(int64(9)).
			WillReturnRows(sqlmock.NewRows([]string{"verified"}).AddRow(true))

		got, err := repo.IsVerified(context.Background(), 9)
		assert.NoError(t, err)
		assert.True(t, got)
	})

	t.Run("user missing", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewVerificationRepository(db)
		mock.ExpectQuery(`SELECT email_verified_at`).WillReturnError(sql.ErrNoRows)

		got, err := repo.IsVerified(context.Background(), 9)
		assert.ErrorIs(t, err, apperror.ErrNotFound)
		assert.False(t, got)
	})

	t.Run("db error", func(t *testing.T) {
		db, mock := setupTestDB(t)
		defer db.Close()
		repo := repository.NewVerificationRepository(db)
		mock.ExpectQuery(`SELECT email_verified_at`).WillReturnError(errDB)

		_, err := repo.IsVerified(context.Background(), 9)
		assert.ErrorIs(t, err, errDB)
	})
}
