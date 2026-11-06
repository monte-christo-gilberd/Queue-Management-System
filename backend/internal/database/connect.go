// Package database berisi pembukaan koneksi ke PostgreSQL memakai sqlx dan driver pgx.
package database

import (
	"context"
	"time"

	// import kosong: mendaftarkan driver "pgx" ke database/sql, sehingga sqlx.Connect("pgx", ...) dikenali
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Config berisi pengaturan koneksi database.
type Config struct {
	// DSN (data source name): string koneksi ke database, mis.
	// postgres://username:password@localhost:5432/qms_db?sslmode=disable
	DSN string
}

// Connect membuka koneksi ke database memakai DSN dari cfg, lalu memastikan database bisa dihubungi (Ping).
// Mengembalikan *sqlx.DB siap pakai, atau error jika gagal. Pemanggil bertanggung jawab memanggil db.Close().
func Connect(cfg Config) (*sqlx.DB, error) {

	// membuka koneksi ke database memakai driver "pgx" dan DSN dari config
	db, err := sqlx.Connect(
		"pgx",
		cfg.DSN,
	)

	// jika error, kembalikan nil sebagai koneksi beserta errornya
	if err != nil {
		return nil, err
	}

	// membuat context dengan batas waktu 5 detik; context ini dipakai oleh PingContext di bawah
	// context = pembawa batas waktu dan pembatalan untuk satu operasi
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)

	// melepas timer context saat fungsi selesai (bukan menutup koneksi; koneksi ditutup pemanggil)
	defer cancel()

	// mengirim ping ke database dengan batas waktu tadi, untuk memastikan database benar-benar bisa dihubungi
	if err := db.PingContext(ctx); err != nil {

		// jika ping gagal, kembalikan error (koneksi yang dibuka di atas tidak ditutup di sini)
		return nil, err
	}

	// koneksi berhasil dibuka dan di-ping
	return db, nil
}
