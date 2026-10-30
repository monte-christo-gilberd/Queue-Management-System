package model

import (
	"time"
)

type Counter struct {
	ID        int64     `db:"id"`
	QueueID   int64     `db:"queue_id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	IsDefault bool      `db:"is_default"`
}
