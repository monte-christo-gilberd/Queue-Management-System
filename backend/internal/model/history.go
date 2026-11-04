package model

import (
	"database/sql"
	"time"
)

// HistoryQueue: ringkasan satu antrian yang sudah ditutup, beserta hitungan tiket per hasil akhir.
type HistoryQueue struct {
	ID           int64     `db:"id"`
	QueueName    string    `db:"queue_name"`
	TicketPrefix string    `db:"ticket_prefix"`
	OpenedAt     time.Time `db:"opened_at"`
	ClosedAt     time.Time `db:"closed_at"`
	Total        int       `db:"total"`
	Completed    int       `db:"completed"`
	Skipped      int       `db:"skipped"`
	Cancelled    int       `db:"cancelled"`
	Deleted      int       `db:"deleted"`
	// Unserved: nomor yang masih menunggu (atau sedang dilayani) saat antrian ditutup.
	Unserved int `db:"unserved"`
}

// HistoryQueueCursor: posisi keyset daftar riwayat (closed_at menurun, lalu id menurun).
type HistoryQueueCursor struct {
	ClosedAt time.Time
	ID       int64
}

// HistoryTicket: satu nomor di antrian yang sudah ditutup. Catatan hanya dibaca owner antrian.
type HistoryTicket struct {
	TicketID      int64          `db:"ticket_id"`
	TicketNumber  int            `db:"ticket_number"`
	Status        string         `db:"status"`
	SkipCount     int            `db:"skip_count"`
	JoinedAt      time.Time      `db:"joined_at"`
	CalledAt      sql.NullTime   `db:"called_at"`
	CompletedAt   sql.NullTime   `db:"completed_at"`
	CancelledAt   sql.NullTime   `db:"cancelled_at"`
	CustomerName  string         `db:"customer_name"`
	CustomerEmail string         `db:"customer_email"`
	CounterName   sql.NullString `db:"counter_name"`
	Note          sql.NullString `db:"note"`
}

// HistoryOutcome menerjemahkan status tiket ke hasil akhir yang ditampilkan di riwayat.
// Nomor yang belum selesai saat antrian ditutup disebut "unserved".
func HistoryOutcome(status string) string {
	switch status {
	case "waiting", "currently_served":
		return "unserved"
	default:
		return status
	}
}

// UserHistoryTicket: satu tiket milik user di riwayat customer. Tanpa catatan owner: catatan tidak pernah
// dilihat customer.
type UserHistoryTicket struct {
	TicketID     int64          `db:"ticket_id"`
	QueueID      int64          `db:"queue_id"`
	QueueName    string         `db:"queue_name"`
	TicketPrefix string         `db:"ticket_prefix"`
	TicketNumber int            `db:"ticket_number"`
	Status       string         `db:"status"`
	QueueStatus  string         `db:"queue_status"`
	SkipCount    int            `db:"skip_count"`
	JoinedAt     time.Time      `db:"joined_at"`
	CalledAt     sql.NullTime   `db:"called_at"`
	CompletedAt  sql.NullTime   `db:"completed_at"`
	CancelledAt  sql.NullTime   `db:"cancelled_at"`
	CounterName  sql.NullString `db:"counter_name"`
}
