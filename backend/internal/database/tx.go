package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

func WithTx(
	ctx context.Context,
	db *sqlx.DB,
	fn func(*sqlx.Tx) error,
) error {

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

type SqlxTxRunner struct {
	db *sqlx.DB
}

func NewSqlxTxRunner(db *sqlx.DB) *SqlxTxRunner {
	return &SqlxTxRunner{
		db: db,
	}
}

func (r *SqlxTxRunner) WithTx(
	ctx context.Context,
	fn func(*sqlx.Tx) error) error {
	return WithTx(
		ctx,
		r.db,
		fn,
	)
}
