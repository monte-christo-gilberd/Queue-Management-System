package service

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type TxRunner interface {
	WithTx(
		ctx context.Context,
		fn func(*sqlx.Tx) error,
	) error
}
