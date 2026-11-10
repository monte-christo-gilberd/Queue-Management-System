package dto

import (
	"qms/internal/model"
	"time"
)

// NoteResponse: catatan satu nomor tiket. Note, CreatedAt, dan UpdatedAt null bila belum ada catatan.
type NoteResponse struct {
	// TicketID: id tiket yang dicatat.
	TicketID int64 `json:"ticket_id"`
	// Note: isi catatan owner. Null bila belum ada catatan.
	Note *string `json:"note"`
	// CreatedAt: waktu catatan pertama kali dibuat. Null bila belum ada catatan.
	CreatedAt *time.Time `json:"created_at"`
	// UpdatedAt: waktu catatan terakhir diubah. Null bila belum ada catatan.
	UpdatedAt *time.Time `json:"updated_at"`
}

// NewNoteResponse membuat NoteResponse dari model.TicketNote. Note dan waktunya null bila tiket belum punya catatan.
func NewNoteResponse(n model.TicketNote) NoteResponse {
	// Salin id tiket dan waktu catatan (nullable lewat nullTimePtr); Note diisi di bawah.
	resp := NoteResponse{
		TicketID: n.TicketID,
		// nullTimePtr: waktu dibuat, kosong bila belum ada catatan.
		CreatedAt: nullTimePtr(n.CreatedAt),
		// nullTimePtr: waktu diubah terakhir, kosong bila belum ada catatan.
		UpdatedAt: nullTimePtr(n.UpdatedAt),
	}
	// Catatan ada (tidak null di database): salin isinya ke variabel baru dan simpan alamatnya.
	if n.Note.Valid {
		note := n.Note.String
		resp.Note = &note
	}
	return resp
}
