package repository

import (
	"context"
	"database/sql"
	"errors"
	"qms/internal/apperror"
	"time"

	"github.com/jmoiron/sqlx"
)

type SubscriptionChecker struct {
	db *sqlx.DB
}

func NewSubscriptionChecker(
	db *sqlx.DB,
) *SubscriptionChecker {
	return &SubscriptionChecker{
		db: db,
	}
}

func (r *SubscriptionChecker) IsActive(
	ctx context.Context,
	ownerID int64,
) (bool, error) {

	var subscriptionExpiresAt time.Time
	const query = `
		SELECT subscription_expires_at
		FROM owners
		WHERE user_id = $1
	`

	err := r.db.GetContext(
		ctx,
		subscriptionExpiresAt,
		query,
		ownerID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return false, apperror.ErrNotFound
	}
	if err != nil {
		return false, err
	}

	if time.Now().Before(subscriptionExpiresAt) {
		return true, nil
	}

	return false, nil
}
