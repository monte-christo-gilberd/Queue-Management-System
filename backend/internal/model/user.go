package model

import (
	"database/sql"
	"time"
)

type User struct {
	ID           int64          `db:"id"`
	Email        string         `db:"email"`
	Username     string         `db:"username"`
	PasswordHash sql.NullString `db:"password_hash"`
	GoogleID     sql.NullString `db:"google_id"`
	CreatedAt    time.Time      `db:"created_at"`
	// EmailNotifications: sakelar email (item 8). Hanya dibaca FindByID; query lain tidak mengisinya.
	EmailNotifications bool `db:"email_notifications"`
	// EmailVerifiedAt: kapan email diverifikasi; tidak valid = belum. Hanya dibaca FindByID dan FindForVerification.
	EmailVerifiedAt sql.NullTime `db:"email_verified_at"`
}
