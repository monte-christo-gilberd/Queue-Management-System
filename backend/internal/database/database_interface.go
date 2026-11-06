package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)
// TxRunnerInterface adalah kontrak untuk menjalankan sekumpulan query dalam satu transaksi.
// Implementasi utamanya ada di tx.go (SqlxTxRunner), dibuat di cmd/server/main.go.
type TxRunnerInterface interface {
	// WithTx menjalankan fn di dalam transaksi. Transaksi di-commit jika fn mengembalikan nil,
	// dan di-rollback jika fn mengembalikan error.
	WithTx(
		ctx context.Context,
		fn func(*sqlx.Tx) error,
	) error
}
