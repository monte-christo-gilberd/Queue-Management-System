package model

import (
	"database/sql"
	"time"
)

// QueueDisplay: konfigurasi layar publik satu antrian. Token ada di URL layar.
//
// Digunakan di fungsi:
//
// DisplayRepository.Get, DisplayService.Config:
//
// - GET /queues/:queue_id/display
//
// Upsert, DisplayService.Configure:
//
// - PUT /queues/:queue_id/display
//
// Rotate, DisplayService.Rotate:
//
// - POST /queues/:queue_id/display/rotate
//
// GetByToken:
//
// - GET /display/:token
type QueueDisplay struct {
	// id antrian
	QueueID int64 `db:"queue_id"`
	// token acak, sebagai kunci akses public display tanpa login
	Token string `db:"token"`
	// maksimum nomor belum dipanggil yang ditampilkan
	UpcomingCount int `db:"upcoming_count"`
	// maksimum nomor di skip yang ditampilkan
	SkippedCount int `db:"skipped_count"`
	// waktu konfigurasi display dibuat
	CreatedAt time.Time `db:"created_at"`
	// waktu token terakhir diganti; NULL bila belum pernah diganti. Token di baris ini selalu yang berlaku
	RotatedAt sql.NullTime `db:"rotated_at"`
}

// DisplayQueueInfo: kolom antrian yang boleh tampil di layar publik.
//
// Digunakan di fungsi:
//
// DisplayRepository.QueueInfo:
//
// - GET /display/:token
type DisplayQueueInfo struct {
	// nama antrian
	Name string `db:"queue_name"`
	// deskripsi antrian
	Desc sql.NullString `db:"queue_desc"`
	// prefiks tiket
	Prefix string `db:"ticket_prefix"`
	// status antrian
	Status string `db:"status"`
}

// DisplayCounter: satu kartu di layar. Serving berisi label siap tampil ("A-007"), kosong bila counter tidak melayani.
//
// Digunakan di fungsi:
//
// build
type DisplayCounter struct {
	// id loket
	ID int64
	// nama loket yang ditampilkan
	Name string
	// tiket yang sedang dilayani loket
	Serving string
}

// DisplaySnapshot: keadaan antrian untuk layar publik. Sengaja hanya berisi teks siap tampil: tanpa id tiket,
// id user, nama customer, atau catatan owner.
//
// Digunakan di fungsi:
//
// build:
//
// - GET /display/:token
type DisplaySnapshot struct {
	// nama antrian
	QueueName string
	// deskripsi antrian, bisa nil
	QueueDesc *string
	// status antrian
	Status string
	// true jika antrian punya lebih dari satu loket
	MultiCounter bool
	// semua display loket
	Counters []DisplayCounter
	// nomor-nomor antrian yang sedang menunggu
	Upcoming []string
	// nomor-nomor antrian yang ter skipped
	Skipped []string
	// waktu snapshot dibuat
	GeneratedAt time.Time
}
