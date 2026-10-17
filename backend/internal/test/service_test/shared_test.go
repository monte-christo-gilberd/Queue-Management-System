package service_test

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type fakeTxRunner struct{}

func (fakeTxRunner) WithTx(
	ctx context.Context,
	fn func(*sqlx.Tx) error,
) error {
	return fn(nil)
}