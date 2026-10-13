package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type TxRunnerInterface interface {
	WithTx(
		ctx context.Context,
		fn func(*sqlx.Tx) error,
	) error
}
