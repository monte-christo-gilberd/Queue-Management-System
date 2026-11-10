package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/store"
)

var ctx = context.Background()

func newMock(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	})
	return sqlx.NewDb(db, "sqlmock"), mock
}

var userCols = []string{"id", "email", "username", "password_hash", "google_id", "created_at"}

// --- UserStore ---

func TestUserStore_Create(t *testing.T) {
	t.Run("sukses mengembalikan user baru", func(t *testing.T) {
		db, mock := newMock(t)
		now := time.Now()
		mock.ExpectQuery(`INSERT INTO users\(email, username, password_hash\)`).
			WithArgs("a@b.c", "abc", "hash").
			WillReturnRows(sqlmock.NewRows(userCols).AddRow(1, "a@b.c", "abc", "hash", nil, now))

		u, err := store.NewUserStore(db).Create(ctx, "a@b.c", "abc", "hash")

		require.NoError(t, err)
		assert.Equal(t, int64(1), u.ID)
		assert.Equal(t, "hash", u.PasswordHash.String)
		assert.False(t, u.GoogleID.Valid)
	})

	t.Run("pelanggaran UNIQUE (23505) menjadi ErrDuplicate", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery(`INSERT INTO users`).WillReturnError(&pgconn.PgError{Code: "23505"})

		_, err := store.NewUserStore(db).Create(ctx, "a@b.c", "abc", "hash")

		assert.ErrorIs(t, err, apperror.ErrDuplicate)
	})

	t.Run("galat lain dibungkus dan bukan ErrDuplicate", func(t *testing.T) {
		db, mock := newMock(t)
		boom := errors.New("koneksi putus")
		mock.ExpectQuery(`INSERT INTO users`).WillReturnError(boom)

		_, err := store.NewUserStore(db).Create(ctx, "a@b.c", "abc", "hash")

		assert.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, apperror.ErrDuplicate)
	})

	t.Run("kode PostgreSQL lain bukan duplikat", func(t *testing.T) {
		db, mock := newMock(t)
		mock.ExpectQuery(`INSERT INTO users`).WillReturnError(&pgconn.PgError{Code: "23502"})

		_, err := store.NewUserStore(db).Create(ctx, "a@b.c", "abc", "hash")

		require.Error(t, err)
		assert.NotErrorIs(t, err, apperror.ErrDuplicate)
	})
}

func TestUserStore_FindByLogin(t *testing.T) {
	db, mock := newMock(t)
	now := time.Now()
	mock.ExpectQuery(`WHERE email = \$1 OR username = \$1\s+ORDER BY \(email = \$1\) DESC, id ASC\s+LIMIT 10`).
		WithArgs("budi").
		WillReturnRows(sqlmock.NewRows(userCols).
			AddRow(1, "a@b.c", "budi", "h1", nil, now).
			AddRow(2, "b@b.c", "budi", nil, "google-sub", now))

	users, err := store.NewUserStore(db).FindByLogin(ctx, "budi")

	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.True(t, users[0].PasswordHash.Valid)
	assert.False(t, users[1].PasswordHash.Valid, "akun Google-only: password NULL")
	assert.Equal(t, "google-sub", users[1].GoogleID.String)
}

func TestUserStore_FindByLogin_NoMatchIsEmptyNotError(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectQuery(`FROM users`).WillReturnRows(sqlmock.NewRows(userCols))

	users, err := store.NewUserStore(db).FindByLogin(ctx, "tidak-ada")

	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestUserStore_FindByEmailAndForVerification_NotFoundMapsToErrNotFound(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectQuery(`SELECT id, email, username FROM users WHERE email = \$1`).WithArgs("x@y.z").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT id, email, username, email_verified_at FROM users WHERE id = \$1`).WithArgs(9).WillReturnError(sql.ErrNoRows)
	s := store.NewUserStore(db)

	_, err := s.FindByEmail(ctx, "x@y.z")
	assert.ErrorIs(t, err, apperror.ErrNotFound)
	_, err = s.FindForVerification(ctx, 9)
	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestUserStore_FindForVerification_ReadsVerifiedAt(t *testing.T) {
	db, mock := newMock(t)
	now := time.Now()
	mock.ExpectQuery(`email_verified_at FROM users`).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "email_verified_at"}).AddRow(1, "a@b.c", "abc", now))
	mock.ExpectQuery(`email_verified_at FROM users`).WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "email_verified_at"}).AddRow(2, "c@d.e", "cde", nil))
	s := store.NewUserStore(db)

	verified, err := s.FindForVerification(ctx, 1)
	require.NoError(t, err)
	assert.True(t, verified.EmailVerifiedAt.Valid)
	unverified, err := s.FindForVerification(ctx, 2)
	require.NoError(t, err)
	assert.False(t, unverified.EmailVerifiedAt.Valid)
}

func TestUserStore_PasswordChangedAt(t *testing.T) {
	db, mock := newMock(t)
	changed := time.Now().Add(-time.Hour).Truncate(time.Second)
	mock.ExpectQuery(`SELECT password_changed_at FROM users WHERE id = \$1`).WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed_at"}).AddRow(changed))
	mock.ExpectQuery(`SELECT password_changed_at FROM users`).WithArgs(2).
		WillReturnRows(sqlmock.NewRows([]string{"password_changed_at"}).AddRow(nil))
	mock.ExpectQuery(`SELECT password_changed_at FROM users`).WithArgs(3).WillReturnError(sql.ErrNoRows)
	boom := errors.New("db down")
	mock.ExpectQuery(`SELECT password_changed_at FROM users`).WithArgs(4).WillReturnError(boom)
	s := store.NewUserStore(db)

	got, err := s.PasswordChangedAt(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.Equal(changed))

	got, err = s.PasswordChangedAt(ctx, 2)
	require.NoError(t, err)
	assert.Nil(t, got, "NULL = password belum pernah diganti")

	_, err = s.PasswordChangedAt(ctx, 3)
	assert.ErrorIs(t, err, apperror.ErrNotFound)

	_, err = s.PasswordChangedAt(ctx, 4)
	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, apperror.ErrNotFound, "galat database bukan 'user tidak ada'")
}

// --- TokenStore ---

func TestTokenStore_CountRecentTokens(t *testing.T) {
	db, mock := newMock(t)
	since := time.Now().Add(-time.Hour)
	mock.ExpectQuery(`SELECT count\(\*\) FROM user_action_tokens WHERE user_id = \$1 AND purpose = \$2 AND created_at > \$3`).
		WithArgs(7, "password_reset", since).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	n, err := store.NewTokenStore(db).CountRecentTokens(ctx, 7, "password_reset", since)

	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestTokenStore_CreateToken_RevokesOldThenInsertsInOneTransaction(t *testing.T) {
	db, mock := newMock(t)
	exp := time.Now().Add(time.Hour)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE user_action_tokens SET used_at = now\(\) WHERE user_id = \$1 AND purpose = \$2 AND used_at IS NULL`).
		WithArgs(7, "email_verify").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO user_action_tokens \(user_id, purpose, token_hash, expires_at, request_ip\)`).
		WithArgs(7, "email_verify", "hash", exp, "1.2.3.4").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := store.NewTokenStore(db).CreateToken(ctx, 7, "email_verify", "hash", exp, "1.2.3.4")

	assert.NoError(t, err)
}

func TestTokenStore_CreateToken_InsertFailureRollsBack(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE user_action_tokens`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`INSERT INTO user_action_tokens`).WillReturnError(errors.New("disk penuh"))
	mock.ExpectRollback()

	err := store.NewTokenStore(db).CreateToken(ctx, 7, "email_verify", "hash", time.Now(), "")

	assert.Error(t, err)
}

func TestTokenStore_ResetPassword_ConsumesTokenSetsPasswordRevokesOthersAtomically(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE user_action_tokens SET used_at = now\(\)\s+WHERE token_hash = \$1 AND purpose = 'password_reset' AND used_at IS NULL AND expires_at > now\(\)\s+RETURNING user_id`).
		WithArgs("tokhash").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
	mock.ExpectExec(`UPDATE users SET password_hash = \$2, password_changed_at = now\(\) WHERE id = \$1`).
		WithArgs(7, "newhash").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE user_action_tokens SET used_at = now\(\) WHERE user_id = \$1 AND purpose = 'password_reset' AND used_at IS NULL`).
		WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	assert.NoError(t, store.NewTokenStore(db).ResetPassword(ctx, "tokhash", "newhash"))
}

func TestTokenStore_ResetPassword_InvalidTokenChangesNothing(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE user_action_tokens`).WithArgs("salah").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback() // password tidak disentuh: tidak ada UPDATE users yang diharapkan

	err := store.NewTokenStore(db).ResetPassword(ctx, "salah", "newhash")

	assert.ErrorIs(t, err, apperror.ErrInvalidResetToken)
}

func TestTokenStore_VerifyEmail_MarksVerifiedWithCoalesceAndRevokesOthers(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE user_action_tokens SET used_at = now\(\)\s+WHERE token_hash = \$1 AND purpose = 'email_verify' AND used_at IS NULL AND expires_at > now\(\)\s+RETURNING user_id`).
		WithArgs("tokhash").WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
	mock.ExpectExec(`UPDATE users SET email_verified_at = COALESCE\(email_verified_at, now\(\)\) WHERE id = \$1`).
		WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE user_action_tokens SET used_at = now\(\) WHERE user_id = \$1 AND purpose = 'email_verify' AND used_at IS NULL`).
		WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	assert.NoError(t, store.NewTokenStore(db).VerifyEmail(ctx, "tokhash"))
}

func TestTokenStore_VerifyEmail_InvalidTokenChangesNothing(t *testing.T) {
	db, mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`UPDATE user_action_tokens`).WithArgs("salah").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	assert.ErrorIs(t, store.NewTokenStore(db).VerifyEmail(ctx, "salah"), apperror.ErrInvalidResetToken)
}
