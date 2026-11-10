package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"
	"time"

	"github.com/jmoiron/sqlx"
)

// PasswordResetRepository: token sekali pakai (user_action_tokens) dan penggantian password.
type PasswordResetRepository struct {
	db *sqlx.DB
}

// NewPasswordResetRepository membuat PasswordResetRepository yang membaca dan menulis lewat *sqlx.DB.
func NewPasswordResetRepository(db *sqlx.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

// FindForReset mencari user berdasarkan email untuk alur reset password. Email tidak ada: ErrNotFound.
func (r *PasswordResetRepository) FindForReset(ctx context.Context, email string) (model.User, error) {
	var u model.User
	// Query ini mengambil id, email, dan username user dengan email tersebut; hasilnya diisi ke u.
	err := r.db.GetContext(ctx, &u, `SELECT id, email, username FROM users WHERE email = $1`, email)
	// Bila tidak ada baris yang cocok, kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	// Error database lain dibungkus dengan konteks 'find user for reset' lalu dikembalikan.
	if err != nil {
		return model.User{}, fmt.Errorf("find user for reset: %w", err)
	}
	return u, nil
}

// CountRecentTokens menghitung token dengan purpose tertentu milik user yang dibuat setelah since.
// Dipakai untuk membatasi frekuensi permintaan reset.
func (r *PasswordResetRepository) CountRecentTokens(ctx context.Context, userID int64, purpose string, since time.Time) (int, error) {
	var n int
	// Query ini menghitung token user dengan purpose ini yang dibuat setelah since; hasilnya diisi ke n.
	err := r.db.GetContext(ctx, &n,
		`SELECT count(*) FROM user_action_tokens WHERE user_id = $1 AND purpose = $2 AND created_at > $3`,
		userID, purpose, since)
	// Bila query gagal, kembalikan 0 dan error yang sudah dibungkus dengan konteks 'count recent tokens'.
	if err != nil {
		return 0, fmt.Errorf("count recent tokens: %w", err)
	}
	return n, nil
}

// CreateToken mencabut token lama dengan purpose yang sama milik user, lalu menyimpan token baru (berupa hash) beserta
// waktu kedaluwarsa dan IP peminta. Keduanya dikerjakan dalam satu transaksi.
func (r *PasswordResetRepository) CreateToken(ctx context.Context, userID int64, purpose, tokenHash string, expiresAt time.Time, requestIP string) error {
	// Mulai transaksi baru agar pencabutan token lama dan penyisipan token baru berjalan atomik.
	tx, err := r.db.BeginTxx(ctx, nil)
	// Bila transaksi gagal dimulai, kembalikan error dengan konteks 'begin'.
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Saat fungsi selesai, transaksi di-rollback; setelah Commit berhasil, panggilan ini tidak berpengaruh.
	defer tx.Rollback()

	// Tandai (used_at = sekarang) semua token dengan purpose ini milik user yang belum terpakai, supaya hanya token baru yang berlaku.
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`,
		userID, purpose); err != nil {
		// Bila gagal, bungkus error dengan konteks 'revoke old tokens' lalu kembalikan.
		return fmt.Errorf("revoke old tokens: %w", err)
	}
	// Query ini menyisipkan token baru (hash, purpose, waktu kedaluwarsa, dan IP peminta) ke tabel user_action_tokens.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_action_tokens (user_id, purpose, token_hash, expires_at, request_ip) VALUES ($1, $2, $3, $4, $5)`,
		userID, purpose, tokenHash, expiresAt, requestIP); err != nil {
		// Bila gagal, bungkus error dengan konteks 'insert token' lalu kembalikan.
		return fmt.Errorf("insert token: %w", err)
	}
	// Simpan kedua perubahan di atas secara permanen; error commit langsung dikembalikan.
	return tx.Commit()
}

// ResetPassword memakai token reset (lewat hash-nya) untuk mengganti password user. Dalam satu transaksi: token
// ditandai terpakai, password diganti, lalu semua token reset lain milik user dicabut. Token tidak valid,
// sudah dipakai, atau kedaluwarsa: ErrInvalidResetToken.
func (r *PasswordResetRepository) ResetPassword(ctx context.Context, tokenHash, passwordHash string) error {
	// Mulai transaksi baru agar seluruh langkah reset berjalan atomik.
	tx, err := r.db.BeginTxx(ctx, nil)
	// Bila transaksi gagal dimulai, kembalikan error dengan konteks 'begin'.
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Saat fungsi selesai, transaksi di-rollback; setelah Commit berhasil, panggilan ini tidak berpengaruh.
	defer tx.Rollback()

	// Satu UPDATE bersyarat: dua permintaan serentak dengan token yang sama, hanya satu yang mendapat baris.
	var userID int64
	// Query ini menandai token terpakai hanya jika cocok dengan hash, purpose password_reset, belum dipakai, dan belum kedaluwarsa, lalu mengembalikan user_id pemiliknya ke userID.
	err = tx.GetContext(ctx, &userID, `
		UPDATE user_action_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = 'password_reset' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash)
	// Tidak ada baris yang berubah (token salah, sudah dipakai, atau kedaluwarsa): kembalikan ErrInvalidResetToken.
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrInvalidResetToken
	}
	// Error database lain dibungkus dengan konteks 'consume token' lalu dikembalikan.
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	// Query ini mengganti password_hash user dan mencatat waktu penggantian password (password_changed_at) sekarang.
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1`, userID, passwordHash); err != nil {
		// Bila gagal, bungkus error dengan konteks 'set password' lalu kembalikan.
		return fmt.Errorf("set password: %w", err)
	}
	// Query ini menandai semua token reset lain milik user yang belum terpakai sebagai terpakai, agar link reset lama tidak bisa dipakai lagi.
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = 'password_reset' AND used_at IS NULL`,
		userID); err != nil {
		// Bila gagal, bungkus error dengan konteks 'revoke other tokens' lalu kembalikan.
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	// Simpan semua perubahan di atas secara permanen; error commit langsung dikembalikan.
	return tx.Commit()
}

// PasswordChangedAt: kapan password terakhir diganti; nil bila belum pernah. apperror.ErrNotFound bila user tidak ada.
// Dipakai middleware untuk mencabut token yang terbit sebelum waktu itu.
func (r *PasswordResetRepository) PasswordChangedAt(ctx context.Context, userID int64) (*time.Time, error) {
	var t sql.NullTime
	// Query ini mengambil kolom password_changed_at milik user; hasilnya diisi ke t (NullTime, karena kolomnya bisa NULL).
	err := r.db.GetContext(ctx, &t, `SELECT password_changed_at FROM users WHERE id = $1`, userID)
	// Bila user tidak ada, kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	// Error database lain dibungkus dengan konteks 'get password_changed_at' lalu dikembalikan.
	if err != nil {
		return nil, fmt.Errorf("get password_changed_at: %w", err)
	}
	// Bila kolom NULL (password belum pernah diganti), kembalikan nil tanpa error.
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}
