package dto_test

import (
	"database/sql"
	"testing"
	"time"

	"qms/internal/dto"
	"qms/internal/model"

	"github.com/stretchr/testify/assert"
)

// Kebocoran catatan owner ke customer adalah risiko privasi: respons tiket biasa tidak boleh memuatnya,
// walau model membawanya.
func TestTicketResponse_NeverIncludesOwnerNote(t *testing.T) {
	ticket := model.Ticket{
		ID:            1,
		Status:        "waiting",
		Note:          sql.NullString{String: "rahasia owner", Valid: true},
		NoteUpdatedAt: sql.NullTime{Time: time.Now(), Valid: true},
	}
	got := asMap(t, dto.NewTicketResponse(ticket))
	assert.NotContains(t, got, "note")
	assert.NotContains(t, got, "note_updated_at")
}

func TestOwnerTicketResponse_IncludesNoteFields(t *testing.T) {
	now := time.Now()
	with := asMap(t, dto.NewOwnerTicketResponse(model.Ticket{
		ID:            1,
		Status:        "completed",
		Note:          sql.NullString{String: "minta struk", Valid: true},
		NoteUpdatedAt: sql.NullTime{Time: now, Valid: true},
	}))
	assert.Equal(t, "minta struk", with["note"])
	assert.NotNil(t, with["note_updated_at"])
	assert.Equal(t, "completed", with["status"], "field tiket biasa tetap ada (embed diratakan)")

	without := asMap(t, dto.NewOwnerTicketResponse(model.Ticket{ID: 2, Status: "waiting"}))
	assert.Contains(t, without, "note", "field note selalu ada, bernilai null")
	assert.Nil(t, without["note"])
	assert.Nil(t, without["note_updated_at"])
}

func TestOwnerTicketResponses_EmptyIsArray(t *testing.T) {
	assert.Equal(t, 0, len(dto.NewOwnerTicketResponses(nil)))
	assert.NotNil(t, dto.NewOwnerTicketResponses(nil))
}
