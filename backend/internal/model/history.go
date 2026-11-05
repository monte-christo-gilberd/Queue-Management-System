package model

import (
	"database/sql"
	"time"
)

// HistoryQueue: ringkasan satu antrian yang sudah ditutup, beserta hitungan tiket per hasil akhir.
//
// Digunakan di fungsi:
//
// ListClosedQueues:
//
// - GET /history/queues
//
// GetClosedQueue:
//
// - GET /history/queues/:queue_id
//
// - GET /history/queues/:queue_id/export?format=csv|xlsx
type HistoryQueue struct {
	// id antrian
	ID int64 `db:"id"`
	// nama antrian
	QueueName string `db:"queue_name"`
	// prefiks tiket
	TicketPrefix string `db:"ticket_prefix"`
	// waktu antrian dibuka
	OpenedAt time.Time `db:"opened_at"`
	// waktu antrian ditutup; bila closed_at kosong, dipakai created_at
	ClosedAt time.Time `db:"closed_at"`
	// total semua tiket yang terdaftar di antrian
	Total int `db:"total"`
	// total semua tiket yang selesai dilayani
	Completed int `db:"completed"`
	// total semua tiket skipped yang tidak terlayani
	Skipped int `db:"skipped"`
	// total semua tiket yang dibatalkan pelanggan
	Cancelled int `db:"cancelled"`
	// total semua tiket yang dihapus pemilik
	Deleted int `db:"deleted"`
	// total nomor yang masih menunggu (atau sedang dilayani) saat antrian ditutup.
	Unserved int `db:"unserved"`
}

// HistoryQueueCursor: posisi keyset daftar riwayat (waktu menurun, lalu id menurun).
//
// Digunakan di fungsi:
//
// encodeQueueCursor dan decodeQueueCursor:
//
// - GET /history/queues
//
// - GET /users/me/history
type HistoryQueueCursor struct {
	// waktu antrian ditutup (endpoint pemilik) / waktu pelanggan bergabung (endpoint pelanggan)
	ClosedAt time.Time
	// id antrian (endpoint pemilik) / id tiket (endpoint pelanggan)
	ID int64
}

// HistoryTicket: satu nomor di antrian yang sudah ditutup. Catatan hanya dibaca owner antrian.
//
// Digunakan di fungsi:
//
// ListQueueTickets:
//
// - GET /history/queues/:queue_id
//
// - GET /history/queues/:queue_id/export
type HistoryTicket struct {
	// id tiket
	TicketID int64 `db:"ticket_id"`
	// nomor tiket
	TicketNumber int `db:"ticket_number"`
	// status tiket saat antrian ditutup
	Status string `db:"status"`
	// berapa kali tiket ini dilewati
	SkipCount int `db:"skip_count"`
	// waktu pelanggan tiket ini bergabung
	JoinedAt time.Time `db:"joined_at"`
	// waktu pelanggan tiket ini dipanggil
	CalledAt sql.NullTime `db:"called_at"`
	// waktu pelanggan tiket ini selesai dilayani
	CompletedAt sql.NullTime `db:"completed_at"`
	// waktu pelanggan tiket ini membatalkan tiketnya
	CancelledAt sql.NullTime `db:"cancelled_at"`
	// nama pelanggan
	CustomerName string `db:"customer_name"`
	// email pelanggan
	CustomerEmail string `db:"customer_email"`
	// nama loket yang melayani tiket ini, hanya terisi jika sudah terpanggil dan loket lebih dari 1
	CounterName sql.NullString `db:"counter_name"`
	// catatan owner tentang tiket ini (hanya terlihat owner)
	Note sql.NullString `db:"note"`
}

// HistoryOutcome menerjemahkan status tiket ke hasil akhir yang ditampilkan di riwayat.
// Nomor yang belum selesai saat antrian ditutup disebut "unserved".
//
// Dipakai di fungsi:
//
// NewHistoryTicketResponse dan NewUserHistoryTicketResponse (GET /history/queues/:queue_id dan GET /users/me/history),
// serta label status di export (export.go).
func HistoryOutcome(status string) string {
	switch status {
	// belum selesai saat antrian ditutup
	case "waiting", "currently_served":
		return "unserved"
	// status lain dipakai apa adanya
	default:
		return status
	}
}

// UserHistoryTicket: satu tiket milik user di riwayat customer. Tanpa catatan owner: catatan tidak pernah
// dilihat customer.
//
// Digunakan di fungsi:
//
// ListUserTickets:
//
// - GET /users/me/history
type UserHistoryTicket struct {
	// id tiket
	TicketID int64 `db:"ticket_id"`
	// id antrian dimana tiket terdaftar
	QueueID int64 `db:"queue_id"`
	// nama antrian tiket terdaftar
	QueueName string `db:"queue_name"`
	// prefiks tiket
	TicketPrefix string `db:"ticket_prefix"`
	// nomor tiket
	TicketNumber int `db:"ticket_number"`
	// status tiket (waiting, currently_served, completed, cancelled, skipped, deleted)
	Status string `db:"status"`
	// status antrian (active, closed)
	QueueStatus string `db:"queue_status"`
	// berapa kali tiket ini dilewati
	SkipCount int `db:"skip_count"`
	// waktu pelanggan tiket ini bergabung antrian
	JoinedAt time.Time `db:"joined_at"`
	// waktu pelanggan tiket ini dipanggil
	CalledAt sql.NullTime `db:"called_at"`
	// waktu pelanggan tiket ini selesai dilayani
	CompletedAt sql.NullTime `db:"completed_at"`
	// waktu pelanggan tiket ini membatalkan
	CancelledAt sql.NullTime `db:"cancelled_at"`
	// nama loket tiket ini dilayani, hanya terisi jika sudah terpanggil dan loket lebih dari 1
	CounterName sql.NullString `db:"counter_name"`
}
