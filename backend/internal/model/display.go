package model

import (
	"database/sql"
	"time"
)

// QueueDisplay: konfigurasi layar publik satu antrian. Token ada di URL layar.
type QueueDisplay struct {
	QueueID       int64        `db:"queue_id"`
	Token         string       `db:"token"`
	UpcomingCount int          `db:"upcoming_count"`
	SkippedCount  int          `db:"skipped_count"`
	CreatedAt     time.Time    `db:"created_at"`
	RotatedAt     sql.NullTime `db:"rotated_at"`
}

// DisplayQueueInfo: kolom antrian yang boleh tampil di layar publik.
type DisplayQueueInfo struct {
	Name   string         `db:"queue_name"`
	Desc   sql.NullString `db:"queue_desc"`
	Prefix string         `db:"ticket_prefix"`
	Status string         `db:"status"`
}

// DisplayCounter: satu kartu di layar. Serving berisi label siap tampil ("A-007"), kosong bila counter tidak melayani.
type DisplayCounter struct {
	ID      int64
	Name    string
	Serving string
}

// DisplaySnapshot: keadaan antrian untuk layar publik. Sengaja hanya berisi teks siap tampil: tanpa id tiket,
// id user, nama customer, atau catatan owner.
type DisplaySnapshot struct {
	QueueName    string
	QueueDesc    *string
	Status       string
	MultiCounter bool
	Counters     []DisplayCounter
	Upcoming     []string
	Skipped      []string
	GeneratedAt  time.Time
}
