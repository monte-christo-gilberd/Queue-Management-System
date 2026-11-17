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

// UserRepository mengakses data akun user di tabel users, serta status owner di tabel owners.
type UserRepository struct {
	// db: koneksi PostgreSQL untuk query di luar transaksi.
	db *sqlx.DB
}

// NewUserRepository membuat UserRepository yang memakai koneksi db.
func NewUserRepository(
	db *sqlx.DB,
) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

// FindByLogin mengembalikan user yang email-nya ATAU username-nya sama dengan identifier.
// Username tidak unik di schema, jadi bisa lebih dari satu baris; kecocokan email didahulukan.
// Hasil dibatasi 10 agar input acak tidak menarik banyak baris. Dipakai fitur sisip customer oleh owner
// (service.AccountFinder); login itu sendiri sekarang ada di auth-service.
func (r *UserRepository) FindByLogin(
	ctx context.Context,
	identifier string,
) ([]model.User, error) {

	const query = `
		SELECT
			id,
			email,
			username,
			password_hash,
			google_id,
			created_at
		FROM users
		WHERE email = $1 OR username = $1
		ORDER BY (email = $1) DESC, id ASC
		LIMIT 10
	`
	var users []model.User

	// Ambil semua user yang cocok; hasilnya diisi ke users.
	if err := r.db.SelectContext(ctx, &users, query, identifier); err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return nil, fmt.Errorf("find user by login: %w", err)
	}
	return users, nil
}

// FindByID mengembalikan user berdasarkan id.
func (r *UserRepository) FindByID(
	ctx context.Context,
	id int64,
) (model.User, error) {

	const query = `
		SELECT
			id,
			email,
			username,
			password_hash,
			google_id,
			created_at,
			email_notifications,
			email_verified_at
		FROM users
		WHERE id = $1
	`
	var user model.User

	// Ambil satu user dengan id tersebut dan isi hasilnya ke user.
	err := r.db.GetContext(ctx, &user, query, id)
	// Tidak ada user dengan id itu; kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	// Error lain dibungkus dengan konteks "find user by id".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}
	return user, nil
}

// SubscriptionExpiresAt: akhir masa langganan owner; nil bila user bukan owner.
func (r *UserRepository) SubscriptionExpiresAt(
	ctx context.Context,
	userID int64,
) (*time.Time, error) {

	const query = `SELECT subscription_expires_at FROM owners WHERE user_id = $1`
	var expiresAt time.Time

	// Ambil subscription_expires_at dari tabel owners untuk user ini dan isi hasilnya ke expiresAt.
	err := r.db.GetContext(ctx, &expiresAt, query, userID)
	// Tidak ada baris di owners, berarti user bukan owner; kembalikan nil tanpa error.
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	// Error lain dibungkus dengan konteks "get subscription expiry".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return nil, fmt.Errorf("get subscription expiry: %w", err)
	}
	return &expiresAt, nil
}

// UpdateUsername mengganti username dan mengembalikan baris user terbaru. Username tidak unik di skema.
func (r *UserRepository) UpdateUsername(
	ctx context.Context,
	userID int64,
	username string,
) (model.User, error) {

	const query = `
		UPDATE users SET username = $2 WHERE id = $1
		RETURNING id, email, username, password_hash, google_id, created_at
	`
	var user model.User

	// Jalankan UPDATE username user dan ambil kembali baris user terbaru (RETURNING) ke user.
	err := r.db.GetContext(ctx, &user, query, userID, username)
	// Tidak ada user dengan id itu, jadi tidak ada baris yang diubah; kembalikan ErrNotFound.
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	// Error lain dibungkus dengan konteks "update username".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return model.User{}, fmt.Errorf("update username: %w", err)
	}
	return user, nil
}

// SetEmailNotifications menyalakan atau mematikan email ke user ini. ErrNotFound bila user tidak ada.
func (r *UserRepository) SetEmailNotifications(
	ctx context.Context,
	userID int64,
	enabled bool,
) error {

	// Jalankan UPDATE kolom email_notifications user dengan nilai enabled.
	res, err := r.db.ExecContext(ctx, `UPDATE users SET email_notifications = $2 WHERE id = $1`, userID, enabled)
	// Jika UPDATE gagal, kembalikan error dengan konteks "set email notifications".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return fmt.Errorf("set email notifications: %w", err)
	}
	// Ambil jumlah baris yang berubah; bila 0, user tidak ada dan kembalikan ErrNotFound. Error dari RowsAffected diabaikan.
	if n, _ := res.RowsAffected(); n == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

// IsOwner: true bila user punya baris di tabel owners (berlangganan atau pernah berlangganan).
func (r *UserRepository) IsOwner(
	ctx context.Context,
	userID int64,
) (bool, error) {

	const query = `SELECT EXISTS (SELECT 1 FROM owners WHERE user_id = $1)`
	var exists bool

	// Jalankan query EXISTS untuk mengecek apakah user punya baris di owners; hasilnya boolean ke exists.
	err := r.db.GetContext(ctx, &exists, query, userID)
	// Jika query gagal, kembalikan false dan error dengan konteks "check owner".
	if err != nil {
		// Bungkus error database dengan pesan konteks; %w menjaga error asli tetap bisa dicek.
		return false, fmt.Errorf("check owner: %w", err)
	}
	return exists, nil
}
