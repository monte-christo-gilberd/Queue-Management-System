package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qms/internal/apperror"
	"time"

	"github.com/jmoiron/sqlx"
)

// SessionRepository membaca kapan password user terakhir diganti, untuk pencabutan sesi. Penggantian password
// dilakukan auth-service; backend hanya membaca, langsung dari database, jadi pencabutan tetap bekerja saat
// auth-service mati.
type SessionRepository struct {
	// db: koneksi PostgreSQL.
	db *sqlx.DB
}

// NewSessionRepository membuat SessionRepository yang memakai koneksi db.
func NewSessionRepository(db *sqlx.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// PasswordChangedAt: kapan password terakhir diganti; nil bila belum pernah. apperror.ErrNotFound bila user tidak ada.
// Dipakai middleware untuk mencabut token yang terbit sebelum waktu itu.
func (r *SessionRepository) PasswordChangedAt(ctx context.Context, userID int64) (*time.Time, error) {
	// NullTime karena kolom NULL bila password belum pernah diganti
	var t sql.NullTime
	err := r.db.GetContext(ctx, &t, `SELECT password_changed_at FROM users WHERE id = $1`, userID)
	// user tidak ada
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperror.ErrNotFound
	}
	// galat lain dibungkus dengan konteks
	if err != nil {
		return nil, fmt.Errorf("get password_changed_at: %w", err)
	}
	// NULL: password belum pernah diganti
	if !t.Valid {
		return nil, nil
	}
	return &t.Time, nil
}
