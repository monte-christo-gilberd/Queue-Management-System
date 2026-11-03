package model

import "database/sql"

// TicketNote: catatan owner untuk satu nomor tiket. Maksimal satu per tiket (ticket_id = PK).
// Note, CreatedAt, dan UpdatedAt null bila tiket belum punya catatan.
type TicketNote struct {
	TicketID  int64          `db:"ticket_id"`
	Note      sql.NullString `db:"note"`
	CreatedAt sql.NullTime   `db:"created_at"`
	UpdatedAt sql.NullTime   `db:"updated_at"`
}

// UpsertedNote: hasil upsert. Created true bila baris baru dibuat, false bila catatan lama ditimpa.
type UpsertedNote struct {
	TicketNote
	Created bool `db:"created"`
}
