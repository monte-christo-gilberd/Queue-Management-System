package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"qms/internal/apperror"
	"qms/internal/model"

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

	const query = `INSERT INTO users(email, username, password_hash) VALUES ($1, $2, $3) RETURNING id, email, username, password_hash, google_id, created_at`
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
			created_at
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
