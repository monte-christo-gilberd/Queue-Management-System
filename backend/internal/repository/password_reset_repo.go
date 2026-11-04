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

func NewPasswordResetRepository(db *sqlx.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) FindForReset(ctx context.Context, email string) (model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `SELECT id, email, username FROM users WHERE email = $1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.User{}, fmt.Errorf("find user for reset: %w", err)
	}
	return u, nil
}

func (r *PasswordResetRepository) CountRecentTokens(ctx context.Context, userID int64, purpose string, since time.Time) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n,
		`SELECT count(*) FROM user_action_tokens WHERE user_id = $1 AND purpose = $2 AND created_at > $3`,
		userID, purpose, since)
	if err != nil {
		return 0, fmt.Errorf("count recent tokens: %w", err)
	}
	return n, nil
}

func (r *PasswordResetRepository) CreateToken(ctx context.Context, userID int64, purpose, tokenHash string, expiresAt time.Time, requestIP string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL`,
		userID, purpose); err != nil {
		return fmt.Errorf("revoke old tokens: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_action_tokens (user_id, purpose, token_hash, expires_at, request_ip) VALUES ($1, $2, $3, $4, $5)`,
		userID, purpose, tokenHash, expiresAt, requestIP); err != nil {
		return fmt.Errorf("insert token: %w", err)
	}
	return tx.Commit()
}

func (r *PasswordResetRepository) ResetPassword(ctx context.Context, tokenHash, passwordHash string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// Satu UPDATE bersyarat: dua permintaan serentak dengan token yang sama, hanya satu yang mendapat baris.
	var userID int64
	err = tx.GetContext(ctx, &userID, `
		UPDATE user_action_tokens SET used_at = now()
		WHERE token_hash = $1 AND purpose = 'password_reset' AND used_at IS NULL AND expires_at > now()
		RETURNING user_id`, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.ErrInvalidResetToken
	}
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1`, userID, passwordHash); err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE user_action_tokens SET used_at = now() WHERE user_id = $1 AND purpose = 'password_reset' AND used_at IS NULL`,
		userID); err != nil {
		return fmt.Errorf("revoke other tokens: %w", err)
	}
	return tx.Commit()
}

// PasswordChangedAt: kapan password terakhir diganti; nil bila belum pernah. apperror.ErrNotFound bila user tidak ada.
// Dipakai middleware untuk mencabut token yang terbit sebelum waktu itu.
func (r *PasswordResetRepository) PasswordChangedAt(ctx context.Context, userID int64) (*time.Time, error) {
	var t sql.NullTime
	err := r.db.GetContext(ctx, &t, `SELECT password_changed_at FROM users WHERE id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get password_changed_at: %w", err)
	}
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}
