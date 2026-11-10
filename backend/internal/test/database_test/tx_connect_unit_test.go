package database_test

import (
	"context"
	"errors"
	"testing"

	"qms/internal/database"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return sqlx.NewDb(db, "sqlmock"), mock
}

func TestWithTx_CommitsWhenFnSucceeds(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	called := false
	err := database.WithTx(context.Background(), db, func(tx *sqlx.Tx) error {
		called = true
		require.NotNil(t, tx)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTx_RollsBackWhenFnFails(t *testing.T) {
	db, mock := newMockDB(t)
	boom := errors.New("boom")
	mock.ExpectBegin()
	mock.ExpectRollback()

	err := database.WithTx(context.Background(), db, func(*sqlx.Tx) error { return boom })

	assert.ErrorIs(t, err, boom)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestWithTx_ReturnsBeginError(t *testing.T) {
	db, mock := newMockDB(t)
	boom := errors.New("no connection")
	mock.ExpectBegin().WillReturnError(boom)

	called := false
	err := database.WithTx(context.Background(), db, func(*sqlx.Tx) error {
		called = true
		return nil
	})

	assert.ErrorIs(t, err, boom)
	assert.False(t, called)
}

func TestWithTx_ReturnsCommitError(t *testing.T) {
	db, mock := newMockDB(t)
	boom := errors.New("commit failed")
	mock.ExpectBegin()
	mock.ExpectCommit().WillReturnError(boom)

	err := database.WithTx(context.Background(), db, func(*sqlx.Tx) error { return nil })

	assert.ErrorIs(t, err, boom)
}

func TestSqlxTxRunner_DelegatesToWithTx(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	runner := database.NewSqlxTxRunner(db)
	called := false
	err := runner.WithTx(context.Background(), func(*sqlx.Tx) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	assert.True(t, called)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnect_FailsOnInvalidDSN(t *testing.T) {
	_, err := database.Connect(database.Config{DSN: "postgres://%zz"})
	assert.Error(t, err)
}

func TestConnect_FailsWhenDatabaseUnreachable(t *testing.T) {
	// port 1 di localhost tidak pernah melayani; ping gagal cepat tanpa menunggu timeout penuh
	db, err := database.Connect(database.Config{
		DSN: "postgres://user:pass@127.0.0.1:1/qms?sslmode=disable&connect_timeout=1",
	})
	assert.Error(t, err)
	assert.Nil(t, db)
}
