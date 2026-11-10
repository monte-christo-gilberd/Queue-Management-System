package model

import (
	"database/sql"
	"time"
)

// User: satu baris tabel users (subset kolom yang dipakai auth-service).
type User struct {
	// ID: id user di database.
	ID int64 `db:"id"`
	// Email: alamat email akun. Dipakai untuk login dan notifikasi.
	Email string `db:"email"`
	// Username: nama tampilan akun. Tidak unik.
	Username string `db:"username"`
	// PasswordHash: hash bcrypt password. Tidak valid (null) untuk akun yang hanya login dengan Google.
	PasswordHash sql.NullString `db:"password_hash"`
	// GoogleID: id akun Google yang ditautkan. Tidak valid (null) bila belum ditautkan.
	GoogleID sql.NullString `db:"google_id"`
	// CreatedAt: waktu akun dibuat.
	CreatedAt time.Time `db:"created_at"`
	// EmailVerifiedAt: kapan email diverifikasi; tidak valid = belum.
	EmailVerifiedAt sql.NullTime `db:"email_verified_at"`
}
