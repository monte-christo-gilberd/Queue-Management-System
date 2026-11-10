package model

import "database/sql"

// TicketNote: catatan owner untuk satu nomor tiket. Maksimal satu per tiket (ticket_id = PK).
// Note, CreatedAt, dan UpdatedAt null bila tiket belum punya catatan.
type TicketNote struct {
	// TicketID: id tiket yang dicatat. Juga kunci utama catatan.
	TicketID int64 `db:"ticket_id"`
	// Note: isi catatan owner. Tidak valid (null) bila belum ada catatan.
	Note sql.NullString `db:"note"`
	// CreatedAt: waktu catatan pertama kali dibuat. Tidak valid (null) bila belum ada catatan.
	CreatedAt sql.NullTime `db:"created_at"`
	// UpdatedAt: waktu catatan terakhir diubah. Tidak valid (null) bila belum ada catatan.
	UpdatedAt sql.NullTime `db:"updated_at"`
}

// UpsertedNote: hasil upsert. Created true bila baris baru dibuat, false bila catatan lama ditimpa.
type UpsertedNote struct {
	// TicketNote: isi catatan yang tersimpan setelah upsert.
	TicketNote
	// Created: true bila catatan baru dibuat, false bila catatan lama ditimpa.
	Created bool `db:"created"`
}
