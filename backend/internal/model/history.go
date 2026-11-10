package model

import (
	"database/sql"
	"time"
)

// HistoryQueue: ringkasan satu antrian yang sudah ditutup, beserta hitungan tiket per hasil akhir.
// Dipakai oleh HistoryRepository.ListClosedQueues (GET /history/queues),
// HistoryRepository.GetClosedQueue (GET /history/queues/:queue_id, dan ekspor
// GET /history/queues/:queue_id/export?format=csv|xlsx).
type HistoryQueue struct {
	// ID: id antrian di database.
	ID int64 `db:"id"`
	// QueueName: nama antrian saat ditutup.
	QueueName string `db:"queue_name"`
	// TicketPrefix: prefix nomor tiket antrian ini.
	TicketPrefix string `db:"ticket_prefix"`
	// OpenedAt: waktu antrian dibuka.
	OpenedAt time.Time `db:"opened_at"`
	// ClosedAt: waktu antrian ditutup. Bila closed_at kosong di database, nilai created_at yang dipakai.
	ClosedAt time.Time `db:"closed_at"`
	// Total: jumlah semua tiket yang terdaftar di antrian.
	Total int `db:"total"`
	// Completed: jumlah tiket yang selesai dilayani.
	Completed int `db:"completed"`
	// Skipped: jumlah tiket yang dilewati dan tidak kembali ke antrian.
	Skipped int `db:"skipped"`
	// Cancelled: jumlah tiket yang dibatalkan customer.
	Cancelled int `db:"cancelled"`
	// Deleted: jumlah tiket yang dihapus owner dari antrian.
	Deleted int `db:"deleted"`
	// Unserved: jumlah nomor yang masih menunggu atau sedang dilayani saat antrian ditutup.
	Unserved int `db:"unserved"`
}

// HistoryQueueCursor: posisi keyset daftar riwayat (waktu menurun, lalu id menurun).
// Dipakai oleh encodeQueueCursor dan decodeQueueCursor (GET /history/queues dan GET /users/me/history).
type HistoryQueueCursor struct {
	// ClosedAt: waktu antrian ditutup (endpoint owner), atau waktu customer bergabung (endpoint customer).
	ClosedAt time.Time
	// ID: id antrian (endpoint owner), atau id tiket (endpoint customer).
	ID int64
}

// HistoryTicket: satu nomor di antrian yang sudah ditutup. Catatan hanya dibaca owner antrian.
// Dipakai oleh HistoryRepository.ListQueueTickets (GET /history/queues/:queue_id dan ekspor
// GET /history/queues/:queue_id/export).
type HistoryTicket struct {
	// TicketID: id tiket di database.
	TicketID int64 `db:"ticket_id"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `db:"ticket_number"`
	// Status: status tiket saat antrian ditutup.
	Status string `db:"status"`
	// SkipCount: berapa kali tiket ini sudah dilewati.
	SkipCount int `db:"skip_count"`
	// JoinedAt: waktu customer bergabung ke antrian.
	JoinedAt time.Time `db:"joined_at"`
	// CalledAt: waktu tiket dipanggil. Tidak valid (null) bila belum pernah dipanggil.
	CalledAt sql.NullTime `db:"called_at"`
	// CompletedAt: waktu nomor selesai dilayani. Tidak valid (null) bila belum selesai.
	CompletedAt sql.NullTime `db:"completed_at"`
	// CancelledAt: waktu customer membatalkan tiketnya. Tidak valid (null) bila tidak dibatalkan.
	CancelledAt sql.NullTime `db:"cancelled_at"`
	// CustomerName: nama customer pemilik tiket.
	CustomerName string `db:"customer_name"`
	// CustomerEmail: email customer pemilik tiket.
	CustomerEmail string `db:"customer_email"`
	// CounterName: nama loket yang melayani tiket ini. Hanya terisi bila sudah terpanggil dan loket lebih dari satu.
	CounterName sql.NullString `db:"counter_name"`
	// Note: catatan owner tentang tiket ini. Hanya terlihat owner. Tidak valid (null) bila belum ada catatan.
	Note sql.NullString `db:"note"`
}

// HistoryOutcome menerjemahkan status tiket ke hasil akhir yang ditampilkan di riwayat.
// Nomor yang belum selesai saat antrian ditutup disebut "unserved".
// Dipakai oleh NewHistoryTicketResponse dan NewUserHistoryTicketResponse (dto), serta label status di export (export.go).
func HistoryOutcome(status string) string {
	switch status {
	// waiting atau currently_served berarti nomor belum selesai saat antrian ditutup: hasilnya unserved.
	case "waiting", "currently_served":
		return "unserved"
	// status lain (completed, skipped, cancelled, deleted) dipakai apa adanya sebagai hasil akhir.
	default:
		return status
	}
}

// UserHistoryTicket: satu tiket milik user di riwayat customer. Tanpa catatan owner: catatan tidak pernah
// dilihat customer.
// Dipakai oleh HistoryRepository.ListUserTickets (GET /users/me/history).
type UserHistoryTicket struct {
	// TicketID: id tiket di database.
	TicketID int64 `db:"ticket_id"`
	// QueueID: id antrian tempat tiket ini terdaftar.
	QueueID int64 `db:"queue_id"`
	// QueueName: nama antrian tempat tiket ini terdaftar.
	QueueName string `db:"queue_name"`
	// TicketPrefix: prefix nomor antrian, mis. "A" di "A-007".
	TicketPrefix string `db:"ticket_prefix"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `db:"ticket_number"`
	// Status: status tiket: waiting, currently_served, completed, cancelled, skipped, atau deleted.
	Status string `db:"status"`
	// QueueStatus: status antrian: active atau closed.
	QueueStatus string `db:"queue_status"`
	// SkipCount: berapa kali tiket ini sudah dilewati.
	SkipCount int `db:"skip_count"`
	// JoinedAt: waktu customer bergabung ke antrian.
	JoinedAt time.Time `db:"joined_at"`
	// CalledAt: waktu tiket dipanggil. Tidak valid (null) bila belum pernah dipanggil.
	CalledAt sql.NullTime `db:"called_at"`
	// CompletedAt: waktu nomor selesai dilayani. Tidak valid (null) bila belum selesai.
	CompletedAt sql.NullTime `db:"completed_at"`
	// CancelledAt: waktu customer membatalkan tiketnya. Tidak valid (null) bila tidak dibatalkan.
	CancelledAt sql.NullTime `db:"cancelled_at"`
	// CounterName: nama loket tempat tiket ini dilayani. Hanya terisi bila sudah terpanggil dan loket lebih dari satu.
	CounterName sql.NullString `db:"counter_name"`
}
