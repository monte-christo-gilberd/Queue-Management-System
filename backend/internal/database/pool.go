package database

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// Nilai bawaan pool koneksi backend. Postgres bawaan memperbolehkan sekitar 100 koneksi untuk SEMUA klien (backend,
// auth-service, mail-service, notify-service), jadi backend dibatasi 40: cukup untuk satu instance dan
// menyisakan ruang bagi yang lain. Tanpa batas, database/sql membuka koneksi sebanyak yang diminta request serentak
// dan menghabiskan slot Postgres saat beban tinggi.
const (
	// defaultMaxOpenConns: batas koneksi terbuka (DB_MAX_OPEN_CONNS).
	defaultMaxOpenConns = 40
	// defaultMaxIdleConns: koneksi menganggur yang dipertahankan (DB_MAX_IDLE_CONNS).
	defaultMaxIdleConns = 10
	// defaultConnMaxLifetime: umur maksimum satu koneksi (DB_CONN_MAX_LIFETIME, detik), supaya koneksi didaur ulang.
	defaultConnMaxLifetime = 30 * time.Minute
)

// PoolSettings membaca pengaturan pool dari environment: DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS, dan
// DB_CONN_MAX_LIFETIME (detik). Nilai kosong, bukan angka, atau tidak positif memakai nilai bawaan (dan dicatat
// sebagai peringatan bila bukan kosong), supaya salah ketik tidak mematikan aplikasi atau membuka pool tanpa batas.
// MaxIdle tidak pernah melebihi MaxOpen.
func PoolSettings() (maxOpen, maxIdle int, maxLifetime time.Duration) {
	// batas koneksi terbuka dan menganggur dari env, dengan nilai bawaan bila tidak valid
	maxOpen = positiveIntEnv("DB_MAX_OPEN_CONNS", defaultMaxOpenConns)
	// koneksi menganggur tidak berguna melebihi koneksi terbuka maksimum
	maxIdle = min(
		positiveIntEnv("DB_MAX_IDLE_CONNS",
			defaultMaxIdleConns,
		),
		maxOpen,
	)
	// umur koneksi dalam detik
	maxLifetime = time.Duration(
		positiveIntEnv(
			"DB_CONN_MAX_LIFETIME",
			int(
				defaultConnMaxLifetime/time.Second,
			),
		),
	) * time.Second
	return maxOpen, maxIdle, maxLifetime
}

// ApplyPool memasang pengaturan pool dari PoolSettings ke db.
func ApplyPool(
	db *sqlx.DB,
) {
	// ambil pengaturan efektif lalu pasang ke pool database/sql
	maxOpen, maxIdle, maxLifetime := PoolSettings()
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
}

// positiveIntEnv membaca bilangan bulat positif dari env. Kosong memakai def tanpa pesan; nilai tidak valid memakai
// def dan mencatat peringatan.
func positiveIntEnv(
	name string,
	def int,
) int {
	// nilai mentah tanpa spasi di pinggir
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	// harus angka positif; selain itu pakai bawaan dan beri tahu operator
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		slog.Warn("invalid database pool setting, using default", "name", name, "value", raw, "default", def)
		return def
	}
	return v
}
