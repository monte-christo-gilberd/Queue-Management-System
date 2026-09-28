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

	const query = `
		INSERT INTO users(email, username, password_hash)
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
