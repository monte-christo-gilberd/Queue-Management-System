package dto

import (
	"qms/internal/model"
	"time"
)

// NoteResponse: catatan satu nomor tiket. Note, CreatedAt, dan UpdatedAt null bila belum ada catatan.
type NoteResponse struct {
	TicketID  int64      `json:"ticket_id"`
	Note      *string    `json:"note"`
	CreatedAt *time.Time `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func NewNoteResponse(n model.TicketNote) NoteResponse {
	resp := NoteResponse{
		TicketID:  n.TicketID,
		CreatedAt: nullTimePtr(n.CreatedAt),
		UpdatedAt: nullTimePtr(n.UpdatedAt),
	}
	if n.Note.Valid {
		note := n.Note.String
		resp.Note = &note
	}
	return resp
}
