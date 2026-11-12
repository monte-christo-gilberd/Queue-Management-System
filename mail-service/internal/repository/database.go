package repository

import (
	"context"
	"log/slog"
	"time"

	// import kosong: mendaftarkan driver "pgx" ke database/sql, sehingga sqlx.Open("pgx", ...) dikenali
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// connectRetryWait: jeda antar percobaan menyambung ke database outbox.
const connectRetryWait = 5 * time.Second

// ConnectWithRetry membuka koneksi ke database outbox dan memastikannya terjangkau (ping). Bila gagal, ia
// mencoba lagi setiap 5 detik sampai berhasil. Mengembalikan nil bila ctx batal sebelum berhasil tersambung.
func ConnectWithRetry(
	ctx context.Context,
	databaseURL string,
) *sqlx.DB {
	// Ulangi sambungan selama konteks belum batal.
	for ctx.Err() == nil {
		// Buka koneksi dan ping sekaligus agar database benar-benar terbukti terjangkau. Pada kegagalan ping,
		// sqlx menutup koneksi yang sudah dibuat.
		db, err := sqlx.ConnectContext(
			ctx,
			"pgx",
			databaseURL,
		)
		
		// Berhasil tersambung: kembalikan koneksi.
		if err == nil {
			return db
		}
		// Gagal tersambung: catat, tunggu 5 detik, lalu coba lagi (kecuali shutdown dimulai).
		slog.Error("outbox database not reachable, retrying", "err", err)
		// Jeda antar percobaan berhenti lebih awal bila shutdown dimulai.
		select {
		case <-ctx.Done():
			// Shutdown: tidak ada koneksi.
			return nil
		case <-time.After(connectRetryWait):
		}
	}
	// Konteks batal sebelum berhasil tersambung: tidak ada koneksi.
	return nil
}
