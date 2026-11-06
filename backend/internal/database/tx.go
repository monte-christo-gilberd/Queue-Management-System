package database

import (
	"context"

	"github.com/jmoiron/sqlx"
)

// WithTx menjalankan fn di dalam satu transaksi database.
// Transaksi di-commit jika fn mengembalikan nil. Jika fn mengembalikan error, transaksi di-rollback dan error diteruskan.
func WithTx(
	ctx context.Context,
	db *sqlx.DB,
	fn func(*sqlx.Tx) error,
) error {

	// memulai transaksi
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	// rollback dijalankan saat fungsi selesai; jika sudah di-commit, rollback tidak berpengaruh
	defer tx.Rollback()

	// menjalankan query di dalam transaksi; error dari fn membuat transaksi di-rollback
	if err := fn(tx); err != nil {
		return err
	}

	// menyimpan semua perubahan ke database
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// SqlxTxRunner membungkus koneksi database agar bisa dipakai untuk menjalankan operasi dalam transaksi.
// Service mendefinisikan interface TxRunner sendiri (lihat internal/service/tx_runner.go), dan SqlxTxRunner memenuhinya.
type SqlxTxRunner struct {
	db *sqlx.DB
}

// NewSqlxTxRunner membuat SqlxTxRunner dengan koneksi database yang diberikan.
func NewSqlxTxRunner(db *sqlx.DB) *SqlxTxRunner {
	return &SqlxTxRunner{
		db: db,
	}
}

// WithTx menjalankan fn dalam transaksi menggunakan koneksi yang tersimpan di SqlxTxRunner.
func (r *SqlxTxRunner) WithTx(
	ctx context.Context,
	fn func(*sqlx.Tx) error,
) error {
	return WithTx(
		ctx,
		r.db,
		fn,
	)
}
