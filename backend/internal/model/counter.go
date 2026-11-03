package model

import (
	"database/sql"
	"time"
)

type Counter struct {
	ID        int64     `db:"id"`
	QueueID   int64     `db:"queue_id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
	IsDefault bool      `db:"is_default"`
}

// CounterWithServing: counter beserta nomor yang sedang dilayaninya (LEFT JOIN tickets). Kosong bila tidak melayani.
type CounterWithServing struct {
	Counter
	ServingTicketID sql.NullInt64 `db:"serving_ticket_id"`
	ServingNumber   sql.NullInt64 `db:"serving_number"`
}
