package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(
	db *sqlx.DB,
) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) Create(
	ctx context.Context,
	email,
	username,
	passwordHash string,
) (model.User, error) {

	const query = `
		INSERT INTO users(
			email, 
			username, 
			password_hash
			) 
		VALUES ($1, $2, $3) 
		RETURNING 
			id, 
			email, 
			username, 
			password_hash, 
			google_id, 
			created_at
	`
	var user model.User

	err := r.db.GetContext(
		ctx,
		&user,
		query,
		email,
		username,
		passwordHash,
	)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.User{}, apperror.ErrDuplicate
	}
	if err != nil {
		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil

}

func (r *UserRepository) FindByEmail(
	ctx context.Context,
	email string,
) (model.User, error) {

	const query = `
		SELECT
			id,
			email,
			username,
			password_hash,
			google_id,
			created_at 
		FROM users 
		WHERE email=$1
	`
	var user model.User

	err := r.db.GetContext(
		ctx,
		&user,
		query,
		email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, apperror.ErrNotFound
		}
		return model.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return user, nil
}

// FindByLogin mengembalikan user yang email-nya ATAU username-nya sama dengan identifier.
// Username tidak unik di schema, jadi bisa lebih dari satu baris; kecocokan email didahulukan.
// Hasil dibatasi 10 agar input acak tidak menarik banyak baris.
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
	users := []model.User{}

	err := r.db.SelectContext(
		ctx,
		&users,
		query,
		identifier,
	)
	if err != nil {
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

	err := r.db.GetContext(ctx, &user, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	if err != nil {
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

	err := r.db.GetContext(ctx, &expiresAt, query, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
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

	err := r.db.GetContext(ctx, &user, query, userID, username)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, apperror.ErrNotFound
	}
	if err != nil {
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

	res, err := r.db.ExecContext(ctx, `UPDATE users SET email_notifications = $2 WHERE id = $1`, userID, enabled)
	if err != nil {
		return fmt.Errorf("set email notifications: %w", err)
	}
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

	err := r.db.GetContext(ctx, &exists, query, userID)
	if err != nil {
		return false, fmt.Errorf("check owner: %w", err)
	}
	return exists, nil
}
