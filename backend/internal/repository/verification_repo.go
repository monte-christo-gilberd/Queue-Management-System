package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qms/internal/apperror"

	"github.com/jmoiron/sqlx"
)

// VerificationRepository membaca status verifikasi email di tabel users. Penulisannya (token, email_verified_at)
// dilakukan auth-service.
type VerificationRepository struct {
	// db: koneksi PostgreSQL.
	db *sqlx.DB
}

// NewVerificationRepository membuat VerificationRepository yang memakai koneksi db.
func NewVerificationRepository(db *sqlx.DB) *VerificationRepository {
	return &VerificationRepository{db: db}
}

// IsVerified: true bila email user sudah diverifikasi; apperror.ErrNotFound bila user tidak ada.
func (r *VerificationRepository) IsVerified(ctx context.Context, userID int64) (bool, error) {
	var verified bool
	// apakah email_verified_at user sudah terisi
	err := r.db.GetContext(ctx, &verified,
		`SELECT email_verified_at IS NOT NULL FROM users WHERE id = $1`, userID)
	// tidak ada user dengan id itu
	if errors.Is(err, sql.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	// galat lain dibungkus dengan konteks
	if err != nil {
		return false, fmt.Errorf("check email verified: %w", err)
	}
	return verified, nil
}
