package model

import (
	"database/sql"
	"time"
)

// Ticket: satu baris tabel tickets, yaitu satu nomor antrian milik seorang customer.
type Ticket struct {
	// ID: id tiket di database.
	ID int64 `db:"id"`
	// QueueID: id antrian tempat tiket ini berada.
	QueueID int64 `db:"queue_id"`
	// TicketNumber: nomor tiket yang dilihat customer (tanpa prefix).
	TicketNumber int `db:"ticket_number"`
	// SortKey: urutan tiket di dalam antrian. Nilai lebih kecil didahulukan.
	SortKey int64 `db:"sort_key"`
	// UserID: id customer pemilik tiket.
	UserID int64 `db:"user_id"`
	// Status: status tiket: waiting, currently_served, completed, skipped, cancelled, atau deleted.
	Status string `db:"status"`
	// SkipCount: berapa kali nomor ini sudah dilewati.
	SkipCount int `db:"skip_count"`
	// LastNotifiedAt: waktu terakhir customer diberi tahu. Tidak valid (null) bila belum pernah.
	LastNotifiedAt sql.NullTime `db:"last_notified_at"`
	// CreatedAt: waktu tiket dibuat (customer bergabung ke antrian).
	CreatedAt time.Time `db:"created_at"`
	// CompletedAt: waktu nomor selesai dilayani. Tidak valid (null) bila belum selesai.
	CompletedAt sql.NullTime `db:"completed_at"`
	// CancelledAt: waktu tiket dibatalkan customer. Tidak valid (null) bila tidak dibatalkan.
	CancelledAt sql.NullTime `db:"cancelled_at"`
	// CalledCounterID: id counter yang memanggil tiket ini (diisi saat Next). 0 = belum dipanggil.
	CalledCounterID int64 `db:"called_counter_id"`
	// CounterName: nama counter yang memanggil tiket ini. Tidak ada di tabel tickets (db:"-"), diisi aplikasi.
	CounterName string `db:"-"`
	// CalledAt: waktu nomor terakhir dipanggil. Tidak valid (null) bila belum pernah dipanggil.
	CalledAt sql.NullTime `db:"called_at"`
	// RecalledAt: waktu owner memanggil ulang nomor ini. Tidak valid (null) bila belum. Boleh sekali per pemanggilan.
	RecalledAt sql.NullTime `db:"recalled_at"`
	// Note: catatan owner untuk nomor ini. Hanya diisi oleh ListByQueue (daftar tiket owner); query lain
	// tidak memilih kolomnya. Jangan pernah dikirim ke customer: NewTicketResponse sengaja tidak memuatnya.
	Note sql.NullString `db:"note"`
	// NoteUpdatedAt: waktu catatan owner terakhir diubah. Hanya diisi bersama Note.
	NoteUpdatedAt sql.NullTime `db:"note_updated_at"`
}
