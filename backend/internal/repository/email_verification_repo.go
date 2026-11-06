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
	db     *sqlx.DB
	tokens *PasswordResetRepository
}

func NewEmailVerificationRepository(
	db *sqlx.DB,
) *EmailVerificationRepository {
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

	err := r.db.GetContext(
		ctx,
		&user,
		query,
		userID,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	if err != nil {
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

	err := r.db.GetContext(
		ctx,
		&verified,
		query,
		userID,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	if err != nil {
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

	tx, err := r.db.BeginTxx(ctx, nil)

	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

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

	err = tx.GetContext(
		ctx,
		&userID,
		useAtQuery,
		tokenHash,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	// COALESCE: akun yang sudah terverifikasi tidak dimajukan waktunya.

	const emailVerifyQuery = `
		UPDATE users 
		SET email_verified_at = COALESCE(email_verified_at, now()) 
		WHERE id = $1
	`

	if _, err := tx.ExecContext(
		ctx,
		emailVerifyQuery,
		userID,
	); err != nil {
		return fmt.Errorf("set email verified: %w", err)
	}

	const userActionQuery = `
		UPDATE user_action_tokens 
		SET used_at = now() 
		WHERE user_id = $1 
		AND purpose = 'email_verify' 
		AND used_at IS NULL
	`

	if _, err := tx.ExecContext(
		ctx,
		userActionQuery,
		userID,
	); err != nil {
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	return tx.Commit()
}
