package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"qms/auth-service/internal/apperror"
)

// TokenRepository mengakses tabel user_action_tokens (hanya hash token yang disimpan).
type TokenRepository struct {
	// db: koneksi PostgreSQL; transaksi dimulai dari sini.
	db *sqlx.DB
}

// NewTokenRepository membuat TokenRepository.
func NewTokenRepository(db *sqlx.DB) *TokenRepository { return &TokenRepository{db: db} }

// CountRecentTokens menghitung token purpose ini milik user yang dibuat setelah since (batas kirim per jam).
func (r *TokenRepository) CountRecentTokens(
	ctx context.Context,
	userID int64,
	purpose string,
	since time.Time,
) (int, error) {
	var n int
	// hitung token user dengan purpose ini yang dibuat setelah since
	err := r.db.GetContext(ctx, &n,
		`SELECT count(*) FROM user_action_tokens WHERE user_id = $1 AND purpose = $2 AND created_at > $3`,
		userID, purpose, since)
	// Query gagal: galat dibungkus dengan konteks.
	if err != nil {
		// Galat database dibungkus dengan konteks.
		return 0, fmt.Errorf("count recent tokens: %w", err)
	}
	// Kembalikan jumlah token yang dihitung.
	return n, nil
}

// CreateToken mencabut token lama (purpose sama) lalu menyimpan token baru, dalam satu transaksi.
func (r *TokenRepository) CreateToken(
	ctx context.Context,
	userID int64,
	purpose,
	tokenHash string,
	expiresAt time.Time,
	requestIP string,
) error {
	// satu transaksi agar pencabutan token lama dan penyisipan token baru berjalan atomik
	tx, err := r.db.BeginTxx(ctx, nil)
	// Transaksi gagal dimulai.
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// rollback otomatis bila keluar sebelum Commit; setelah Commit berhasil tidak berpengaruh
	defer tx.Rollback()
	// tandai token lama (purpose sama) yang belum dipakai sebagai terpakai, agar hanya token baru yang berlaku
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`,
		userID, purpose); err != nil {
		// Gagal mencabut token lama: transaksi dibatalkan oleh defer Rollback.
		return fmt.Errorf("revoke old tokens: %w", err)
	}
	// simpan token baru (hash, purpose, kedaluwarsa, IP peminta)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_action_tokens (user_id, purpose, token_hash, expires_at, request_ip) VALUES ($1, $2, $3, $4, $5)`,
		userID, purpose, tokenHash, expiresAt, requestIP); err != nil {
		// Gagal menyimpan token baru: pencabutan token lama ikut dibatalkan.
		return fmt.Errorf("insert token: %w", err)
	}
	// Commit membuat pencabutan dan penyisipan berlaku bersamaan.
	return tx.Commit()
}

// ResetPassword memakai token reset: token ditandai terpakai, password diganti, token reset lain dicabut (atomik).
// Token tidak sah, sudah dipakai, atau kedaluwarsa: ErrInvalidResetToken.
func (r *TokenRepository) ResetPassword(
	ctx context.Context,
	tokenHash,
	passwordHash string,
) error {
	// Mulai transaksi agar penandaan token dan penggantian password atomik.
	tx, err := r.db.BeginTxx(ctx, nil)
	// Transaksi gagal dimulai.
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Batalkan semua perubahan bila fungsi keluar sebelum Commit.
	defer tx.Rollback()

	// Satu UPDATE bersyarat: dua permintaan serentak dengan token yang sama, hanya satu yang mendapat baris.
	var userID int64
	// Query ini menandai token reset yang masih berlaku (belum dipakai, belum kedaluwarsa) sebagai terpakai dan mengembalikan pemiliknya.
	err = tx.GetContext(ctx, &userID, `
		UPDATE user_action_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = 'password_reset' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash)
	// tidak ada baris yang berubah: token salah, sudah dipakai, atau kedaluwarsa
	if errors.Is(err, sql.ErrNoRows) {
		// Dipetakan ke ErrInvalidResetToken agar penyebab tidak dibedakan ke pengguna.
		return apperror.ErrInvalidResetToken
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	// ganti password dan catat waktunya; token login yang terbit sebelumnya jadi dicabut
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1`, userID, passwordHash); err != nil {
		// Gagal mengganti password: penandaan token ikut dibatalkan.
		return fmt.Errorf("set password: %w", err)
	}
	// cabut semua token reset lain milik user agar tautan lama tidak bisa dipakai lagi
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = 'password_reset' AND used_at IS NULL`,
		userID); err != nil {
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	// Commit menyimpan semua perubahan sekaligus.
	return tx.Commit()
}

// VerifyEmail memakai token verifikasi: token ditandai terpakai, email_verified_at diisi, token lain dicabut (atomik).
// Token tidak sah, sudah dipakai, atau kedaluwarsa: ErrInvalidResetToken. Mengembalikan id pemilik token dan
// newlyVerified (true bila email_verified_at baru terisi oleh token ini).
func (r *TokenRepository) VerifyEmail(
	ctx context.Context,
	tokenHash string,
) (userID int64, newlyVerified bool, err error) {
	// Mulai transaksi agar penandaan token dan pengisian email_verified_at atomik.
	tx, err := r.db.BeginTxx(ctx, nil)
	// Transaksi gagal dimulai.
	if err != nil {
		return 0, false, fmt.Errorf("begin: %w", err)
	}
	// Batalkan semua perubahan bila fungsi keluar sebelum Commit.
	defer tx.Rollback()

	// Query ini menandai token verifikasi yang masih berlaku sebagai terpakai dan mengembalikan pemiliknya.
	err = tx.GetContext(ctx, &userID, `
		UPDATE user_action_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = 'email_verify' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash)
	// token salah, sudah dipakai, atau kedaluwarsa
	if errors.Is(err, sql.ErrNoRows) {
		// Dipetakan ke ErrInvalidResetToken.
		return 0, false, apperror.ErrInvalidResetToken
	}
	// Galat database lain dibungkus dengan konteks.
	if err != nil {
		return 0, false, fmt.Errorf("consume token: %w", err)
	}
	// COALESCE: akun yang sudah terverifikasi tidak dimajukan waktunya.
	res, err := tx.ExecContext(ctx,
		`UPDATE users SET email_verified_at = COALESCE(email_verified_at, now()) WHERE id = $1 AND email_verified_at IS NULL`, userID)
	// Gagal mengisi email_verified_at: penandaan token ikut dibatalkan.
	if err != nil {
		return 0, false, fmt.Errorf("set email verified: %w", err)
	}
	// baris terubah berarti akun baru terverifikasi sekarang; nol berarti sudah terverifikasi sebelumnya
	changed, err := res.RowsAffected()
	// Jumlah baris terubah tidak terbaca.
	if err != nil {
		return 0, false, fmt.Errorf("rows affected: %w", err)
	}
	// cabut token verifikasi lain milik user
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = 'email_verify' AND used_at IS NULL`,
		userID); err != nil {
		return 0, false, fmt.Errorf("revoke other tokens: %w", err)
	}
	// Commit menyimpan semua perubahan sekaligus.
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	// Token terpakai: kembalikan pemilik dan status verifikasi baru.
	return userID, changed > 0, nil
}
