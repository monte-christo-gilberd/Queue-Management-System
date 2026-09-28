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
}
