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
	// db adalah koneksi PostgreSQL yang dipakai untuk membaca dan menulis foto profil.
	db *sqlx.DB
}

// NewAvatarRepository membuat AvatarRepository baru dengan koneksi database yang diberikan.
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
	// Jalankan query upsert untuk menyimpan foto baru (atau menimpa yang lama) dan mengambil updated_at-nya; jika gagal, bungkus error dengan konteks "put avatar".
	if err := r.db.GetContext(ctx, &updatedAt, query, userID, data); err != nil {
		// Kembalikan waktu kosong bersama error yang sudah diberi konteks, supaya pemanggil tahu penyimpanan gagal.
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
	// Jalankan query yang mengambil data foto, tipe konten, dan waktu update milik user ini, lalu isi ke row.
	err := r.db.GetContext(ctx, &row, query, userID)
	// Jika tidak ada baris foto untuk user ini, kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.Avatar{}, apperror.ErrNotFound
	}
	// Jika terjadi error database lain, bungkus dengan konteks "get avatar" lalu kembalikan.
	if err != nil {
		return model.Avatar{}, fmt.Errorf("get avatar: %w", err)
	}
	return model.Avatar{Data: row.Data, ContentType: row.ContentType, UpdatedAt: row.UpdatedAt}, nil
}

// Delete menghapus foto user. Idempoten: tidak ada foto bukan galat.
func (r *AvatarRepository) Delete(ctx context.Context, userID int64) error {
	// Jalankan DELETE untuk menghapus foto milik user ini; jika gagal, bungkus error dengan konteks "delete avatar" lalu kembalikan.
	if _, err := r.db.ExecContext(ctx, `DELETE FROM user_avatars WHERE user_id = $1`, userID); err != nil {
		// Teruskan error yang sudah diberi konteks ke pemanggil.
		return fmt.Errorf("delete avatar: %w", err)
	}
	return nil
}

// UpdatedAt: waktu foto terakhir diganti; nil bila belum ada. Tidak membaca kolom data.
func (r *AvatarRepository) UpdatedAt(ctx context.Context, userID int64) (*time.Time, error) {
	var t time.Time
	// Jalankan query yang hanya mengambil updated_at (tanpa kolom data foto) milik user ini, lalu isi ke t.
	err := r.db.GetContext(ctx, &t, `SELECT updated_at FROM user_avatars WHERE user_id = $1`, userID)
	// Jika user belum punya foto, kembalikan nil (bukan error) sesuai kontrak fungsi.
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	// Jika terjadi error database lain, bungkus dengan konteks "avatar updated_at" lalu kembalikan.
	if err != nil {
		return nil, fmt.Errorf("avatar updated_at: %w", err)
	}
	return &t, nil
}
