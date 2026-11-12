package database

import (
	"context"
	"time"

	// import kosong: mendaftarkan driver "pgx" ke database/sql, sehingga sqlx.Connect("pgx", ...) dikenali
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Connect membuka koneksi dan memastikan database terjangkau (Ping, 5 detik).
func Connect(
	dsn string,
) (*sqlx.DB, error) {
	// buka koneksi dengan driver pgx (didaftarkan oleh import kosong stdlib di atas)
	db, err := sqlx.Connect("pgx", dsn)
	// Gagal terhubung: kembalikan galat ke pemanggil.
	if err != nil {
		// Gagal terhubung: tidak ada koneksi untuk dikembalikan.
		return nil, err
	}
	// batasi pool koneksi (DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS, DB_CONN_MAX_LIFETIME); bawaan auth-service lebih kecil
	// dari backend karena jalur login dibatasi bcrypt dan rate limit
	ApplyPool(db)
	// batas 5 detik untuk ping; melepas timer saat fungsi selesai (bukan menutup koneksi)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// Lepas timer saat fungsi selesai (bukan menutup koneksi).
	defer cancel()
	// pastikan database benar-benar bisa dihubungi
	if err := db.PingContext(ctx); err != nil {
		// Ping gagal: koneksi tidak dikembalikan agar tidak dipakai.
		return nil, err
	}
	// koneksi siap dipakai; pemanggil yang menutupnya
	return db, nil
}
