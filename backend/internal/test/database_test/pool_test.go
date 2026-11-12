package database_test

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/internal/database"
)

// Tanpa env, pool memakai nilai bawaan yang membatasi koneksi (bukan tanpa batas seperti bawaan database/sql).
func TestPoolSettings_Defaults(t *testing.T) {
	// kosongkan env supaya hanya nilai bawaan yang berlaku
	t.Setenv("DB_MAX_OPEN_CONNS", "")
	t.Setenv("DB_MAX_IDLE_CONNS", "")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")

	open, idle, life := database.PoolSettings()

	// 40 terbuka, 10 menganggur, umur 30 menit
	assert.Equal(t, 40, open)
	assert.Equal(t, 10, idle)
	assert.Equal(t, 30*time.Minute, life)
}

// Nilai dari env dipakai, dan umur koneksi dibaca dalam detik.
func TestPoolSettings_FromEnv(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "80")
	t.Setenv("DB_MAX_IDLE_CONNS", "20")
	t.Setenv("DB_CONN_MAX_LIFETIME", "600")

	open, idle, life := database.PoolSettings()

	assert.Equal(t, 80, open)
	assert.Equal(t, 20, idle)
	assert.Equal(t, 10*time.Minute, life)
}

// Nilai tidak valid (bukan angka, nol, negatif) jatuh ke nilai bawaan: salah ketik tidak boleh membuka pool tanpa batas
// (database/sql menganggap 0 sebagai tanpa batas).
func TestPoolSettings_InvalidFallsBackToDefault(t *testing.T) {
	for _, bad := range []string{"abc", "0", "-5", "1.5"} {
		t.Setenv("DB_MAX_OPEN_CONNS", bad)
		t.Setenv("DB_MAX_IDLE_CONNS", "")
		t.Setenv("DB_CONN_MAX_LIFETIME", "")

		open, _, _ := database.PoolSettings()

		assert.Equal(t, 40, open, "nilai %q harus memakai bawaan", bad)
	}
}

// Koneksi menganggur tidak boleh melebihi koneksi terbuka maksimum (SetMaxIdleConns dibatasi database/sql, tetapi
// nilai efektif dibuat konsisten di sini supaya yang dilaporkan sama dengan yang dipakai).
func TestPoolSettings_IdleCappedByOpen(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "5")
	t.Setenv("DB_MAX_IDLE_CONNS", "50")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")

	open, idle, _ := database.PoolSettings()

	assert.Equal(t, 5, open)
	assert.Equal(t, 5, idle)
}

// ApplyPool benar-benar memasang batas ke pool database/sql (dibaca kembali lewat Stats).
func TestApplyPool_SetsMaxOpenConnections(t *testing.T) {
	t.Setenv("DB_MAX_OPEN_CONNS", "7")
	t.Setenv("DB_MAX_IDLE_CONNS", "3")
	t.Setenv("DB_CONN_MAX_LIFETIME", "")
	// sqlmock menyediakan *sql.DB tanpa database sungguhan
	raw, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { raw.Close() })
	db := sqlx.NewDb(raw, "sqlmock")

	database.ApplyPool(db)

	// batas koneksi terbuka terbaca di statistik pool
	assert.Equal(t, 7, db.Stats().MaxOpenConnections)
}
