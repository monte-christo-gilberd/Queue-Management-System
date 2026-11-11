package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connectRetryWait: jeda antar percobaan menyambung ke database outbox.
const connectRetryWait = 5 * time.Second

// ConnectWithRetry membuka pool koneksi ke database outbox dan memastikannya terjangkau (ping). Bila gagal, ia
// mencoba lagi setiap 5 detik sampai berhasil. Mengembalikan nil bila ctx batal sebelum berhasil tersambung.
func ConnectWithRetry(
	ctx context.Context,
	databaseURL string,
) *pgxpool.Pool {
	// Ulangi sambungan selama konteks belum batal.
	for ctx.Err() == nil {
		// Buat pool, lalu ping untuk memastikan database benar-benar bisa dijangkau.
		p, err := pgxpool.New(ctx, databaseURL)
		// Pool dibuat malas, jadi Ping yang membuktikan database terjangkau.
		if err == nil {
			err = p.Ping(ctx)
			// Ping gagal: tutup pool yang sudah dibuat supaya tidak bocor.
			if err != nil {
				p.Close()
			}
		}
		// Berhasil tersambung: kembalikan pool.
		if err == nil {
			// Pool siap dipakai.
			return p
		}
		// Gagal tersambung: catat, tunggu 5 detik, lalu coba lagi (kecuali shutdown dimulai).
		slog.Error("outbox database not reachable, retrying", "err", err)
		// Jeda antar percobaan berhenti lebih awal bila shutdown dimulai.
		select {
		case <-ctx.Done():
			// Shutdown: tidak ada pool.
			return nil
		case <-time.After(connectRetryWait):
		}
	}
	// Konteks batal sebelum berhasil tersambung: tidak ada pool.
	return nil
}
