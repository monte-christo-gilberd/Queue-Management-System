package model

import (
	"database/sql"
	"time"
)

type Counter struct {
	// id loket
	ID int64 `db:"id"`
	// id antrian
	QueueID int64 `db:"queue_id"`
	// nama loket
	Name string `db:"name"`
	// waktu loket dibuat
	CreatedAt time.Time `db:"created_at"`
	// apakah loket ini loket default
	IsDefault bool `db:"is_default"`
}

// CounterWithServing: counter beserta nomor yang sedang dilayaninya (LEFT JOIN tickets). Kosong bila tidak melayani.
type CounterWithServing struct {
	// counternya sendiri
	Counter
	// id tiket yang sedang dilayani
	ServingTicketID sql.NullInt64 `db:"serving_ticket_id"`
	// nomor yang sedang dilayani
	ServingNumber sql.NullInt64 `db:"serving_number"`
}
