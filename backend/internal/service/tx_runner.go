package service

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// TxRunner menjalankan fn dalam satu transaksi: commit bila fn mengembalikan nil, rollback bila error.
// Dipenuhi database.SqlxTxRunner (lihat package database, tx.go).
type TxRunner interface {
	WithTx(
		ctx context.Context,
		fn func(*sqlx.Tx) error,
	) error
}
