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

// EmailVerificationRepository: status verifikasi email di users dan token sekali pakai (user_action_tokens,
// purpose 'email_verify'). Pembuatan token dan penghitungan batas per jam dipakai bersama dengan reset password.
type EmailVerificationRepository struct {
	// db: koneksi PostgreSQL untuk query status verifikasi di tabel users.
	db *sqlx.DB
	// tokens: repository token bersama (PasswordResetRepository) untuk membuat dan menghitung token verifikasi.
	tokens *PasswordResetRepository
}

// NewEmailVerificationRepository membuat EmailVerificationRepository yang memakai koneksi db dan PasswordResetRepository untuk token.
func NewEmailVerificationRepository(
	db *sqlx.DB,
) *EmailVerificationRepository {
	// Buat repository dengan koneksi db, dan PasswordResetRepository baru yang dipakai bersama untuk urusan token.
	return &EmailVerificationRepository{
		db:     db,
		tokens: NewPasswordResetRepository(db),
	}
}

// FindForVerification: id, email, username, dan status verifikasi user; apperror.ErrNotFound bila tidak ada.
func (r *EmailVerificationRepository) FindForVerification(
	ctx context.Context,
	userID int64,
) (model.User, error) {

	var user model.User

	const query = `
		SELECT
			id,
			email,
			username,
			email_verified_at
		FROM users
		WHERE id = $1
	`

	// Ambil id, email, username, dan email_verified_at user dengan id userID; hasilnya diisi ke user.
	err := r.db.GetContext(
		ctx,
		&user,
		query,
		userID,
	)

	// Tidak ada user dengan id itu; kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	// Error lain dibungkus dengan konteks "find user for verification".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return model.User{},
			fmt.Errorf("find user for verification: %w", err)
	}
	return user, nil
}

// IsVerified: true bila email user sudah diverifikasi; apperror.ErrNotFound bila user tidak ada.
func (r *EmailVerificationRepository) IsVerified(
	ctx context.Context,
	userID int64,
) (bool, error) {

	const query = `
		SELECT
			email_verified_at IS NOT NULL
		FROM users
		WHERE id = $1
	`

	var verified bool

	// Jalankan query yang menghasilkan boolean: apakah email_verified_at user sudah terisi; hasilnya ke verified.
	err := r.db.GetContext(
		ctx,
		&verified,
		query,
		userID,
	)

	// Tidak ada user dengan id itu; kembalikan false dan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	// Error lain dibungkus dengan konteks "check email verified".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return false,
			fmt.Errorf("check email verified: %w", err)
	}
	return verified, nil
}

// CountRecentTokens: jumlah token purpose ini yang dibuat sejak waktu since (batas kirim per jam).
func (r *EmailVerificationRepository) CountRecentTokens(
	ctx context.Context,
	userID int64,
	purpose string,
	since time.Time,
) (int, error) {
	// Delegasikan penghitungan ke PasswordResetRepository, karena tabel token dipakai bersama.
	return r.tokens.CountRecentTokens(
		ctx,
		userID,
		purpose,
		since,
	)
}

// CreateToken menyimpan token baru dan membatalkan token lama yang belum terpakai (purpose sama) dalam satu transaksi.
func (r *EmailVerificationRepository) CreateToken(
	ctx context.Context,
	userID int64,
	purpose,
	tokenHash string,
	expiresAt time.Time,
	requestIP string,
) error {
	// Delegasikan pembuatan token ke PasswordResetRepository, agar logika token tidak diduplikasi.
	return r.tokens.CreateToken(
		ctx,
		userID,
		purpose,
		tokenHash,
		expiresAt,
		requestIP,
	)
}

// VerifyEmail menandai token terpakai, mengisi email_verified_at, dan membatalkan token verifikasi lain milik user,
// semuanya atomik. apperror.ErrInvalidResetToken bila token tidak ada, kedaluwarsa, atau sudah dipakai.
func (r *EmailVerificationRepository) VerifyEmail(
	ctx context.Context,
	tokenHash string,
) error {

	// Mulai transaksi agar ketiga langkah verifikasi berhasil atau gagal bersama.
	tx, err := r.db.BeginTxx(ctx, nil)

	// Jika gagal memulai transaksi, kembalikan error dengan konteks "begin".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return fmt.Errorf("begin: %w", err)
	}

	// Jika fungsi keluar sebelum Commit, transaksi di-rollback; setelah Commit berhasil, panggilan ini tidak berpengaruh.
	defer tx.Rollback()

	// Satu UPDATE bersyarat: dua permintaan serentak dengan token yang sama, hanya satu yang mendapat baris.
	var userID int64

	const useAtQuery = `
		UPDATE user_action_tokens
		SET used_at = now()
		WHERE
			token_hash = $1
		AND purpose = 'email_verify'
		AND used_at IS NULL
		AND expires_at > now()
		RETURNING user_id
	`

	// Jalankan UPDATE yang menandai token terpakai dan mengambil user_id pemilik token ke userID.
	err = tx.GetContext(
		ctx,
		&userID,
		useAtQuery,
		tokenHash,
	)

	// Tidak ada token yang cocok (salah, kedaluwarsa, atau sudah dipakai); kembalikan ErrInvalidResetToken.
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrInvalidResetToken
	}
	// Error lain saat mengonsumsi token dikembalikan dengan konteks "consume token".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return fmt.Errorf("consume token: %w", err)
	}
	// COALESCE: akun yang sudah terverifikasi tidak dimajukan waktunya.

	const emailVerifyQuery = `
		UPDATE users
		SET email_verified_at = COALESCE(email_verified_at, now())
		WHERE id = $1
	`

	// Jalankan UPDATE untuk mengisi email_verified_at user jika belum terisi; error dikembalikan bila gagal.
	if _, err := tx.ExecContext(
		ctx,
		emailVerifyQuery,
		userID,
	); err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return fmt.Errorf("set email verified: %w", err)
	}

	const userActionQuery = `
		UPDATE user_action_tokens
		SET used_at = now()
		WHERE user_id = $1
		AND purpose = 'email_verify'
		AND used_at IS NULL
	`

	// Tandai semua token verifikasi email lain milik user ini sebagai terpakai; error dikembalikan bila gagal.
	if _, err := tx.ExecContext(
		ctx,
		userActionQuery,
		userID,
	); err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	// Simpan semua perubahan; bila Commit gagal, error dikembalikan.
	return tx.Commit()
}
