package model

import (
	"database/sql"
	"time"
)

type Queue struct {
	ID           int64          `db:"id"`
	OwnerID      int64          `db:"owner_id"`
	QueueName    string         `db:"queue_name"`
	QueueDesc    sql.NullString `db:"queue_desc"`
	TicketPrefix string         `db:"ticket_prefix"`
	Status       string         `db:"status"`
	CreatedAt    time.Time      `db:"created_at"`
	ClosedAt     sql.NullTime   `db:"closed_at"`
}
