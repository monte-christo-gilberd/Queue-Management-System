package model

import (
	"database/sql"
	"time"
)

type Ticket struct {
	ID             int64        `db:"id"`
	QueueID        int64        `db:"queue_id"`
	TicketNumber   int          `db:"ticket_number"`
	SortKey        int64        `db:"sort_key"`
	UserID         int64        `db:"user_id"`
	Status         string       `db:"status"`
	SkipCount      int          `db:"skip_count"`
	LastNotifiedAt sql.NullTime `db:"last_notified_at"`
	CreatedAt      time.Time    `db:"created_at"`
	CompletedAt    sql.NullTime `db:"completed_at"`
	CancelledAt    sql.NullTime `db:"cancelled_at"`
	// Counter yang memanggil tiket ini (diisi saat Next); 0 = belum dipanggil.
	CalledCounterID int64        `db:"called_counter_id"`
	CounterName     string       `db:"-"`
	CalledAt        sql.NullTime `db:"called_at"`
	// Kapan owner memanggil ulang nomor ini; NULL = belum (boleh sekali per pemanggilan).
	RecalledAt sql.NullTime `db:"recalled_at"`
}
