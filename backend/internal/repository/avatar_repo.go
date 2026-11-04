package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"qms/internal/apperror"
	"qms/internal/model"

	"github.com/jmoiron/sqlx"
)

// AvatarRepository menyimpan foto profil di tabel user_avatars (bytea). Implementasi service.AvatarStore.
type AvatarRepository struct {
	db *sqlx.DB
}

func NewAvatarRepository(db *sqlx.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

// Put mengganti foto user (upsert) dan mengembalikan waktu pembaruannya.
func (r *AvatarRepository) Put(ctx context.Context, userID int64, data []byte) (time.Time, error) {
	const query = `
		INSERT INTO user_avatars (user_id, data) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data, updated_at = now()
		RETURNING updated_at
	`
	var updatedAt time.Time
	if err := r.db.GetContext(ctx, &updatedAt, query, userID, data); err != nil {
		return time.Time{}, fmt.Errorf("put avatar: %w", err)
	}
	return updatedAt, nil
}

// Get mengembalikan foto user, atau ErrNotFound bila belum ada.
func (r *AvatarRepository) Get(ctx context.Context, userID int64) (model.Avatar, error) {
	const query = `SELECT data, content_type, updated_at FROM user_avatars WHERE user_id = $1`
	var row struct {
		Data        []byte    `db:"data"`
		ContentType string    `db:"content_type"`
		UpdatedAt   time.Time `db:"updated_at"`
	}
	err := r.db.GetContext(ctx, &row, query, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Avatar{}, apperror.ErrNotFound
	}
	if err != nil {
		return model.Avatar{}, fmt.Errorf("get avatar: %w", err)
	}
	return model.Avatar{Data: row.Data, ContentType: row.ContentType, UpdatedAt: row.UpdatedAt}, nil
}

// Delete menghapus foto user. Idempoten: tidak ada foto bukan galat.
func (r *AvatarRepository) Delete(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM user_avatars WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete avatar: %w", err)
	}
	return nil
}

// UpdatedAt: waktu foto terakhir diganti; nil bila belum ada. Tidak membaca kolom data.
func (r *AvatarRepository) UpdatedAt(ctx context.Context, userID int64) (*time.Time, error) {
	var t time.Time
	err := r.db.GetContext(ctx, &t, `SELECT updated_at FROM user_avatars WHERE user_id = $1`, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("avatar updated_at: %w", err)
	}
	return &t, nil
}
