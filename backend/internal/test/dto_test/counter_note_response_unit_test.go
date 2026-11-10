package dto_test

import (
	"database/sql"
	"testing"
	"time"

	"qms/internal/dto"
	"qms/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestNewCounterResponse_ServingFilledWhenBothValid(t *testing.T) {
	got := dto.NewCounterResponse(model.CounterWithServing{
		Counter:         model.Counter{ID: 2, Name: "Loket 1", IsDefault: true},
		ServingTicketID: sql.NullInt64{Int64: 40, Valid: true},
		ServingNumber:   sql.NullInt64{Int64: 12, Valid: true},
	})
	assert.Equal(t, int64(2), got.ID)
	assert.Equal(t, "Loket 1", got.Name)
	assert.True(t, got.IsDefault)
	if assert.NotNil(t, got.Serving) {
		assert.Equal(t, int64(40), got.Serving.TicketID)
		assert.Equal(t, int64(12), got.Serving.TicketNumber)
	}
}

func TestNewCounterResponse_NoServingWhenEitherNull(t *testing.T) {
	idOnly := dto.NewCounterResponse(model.CounterWithServing{
		Counter:         model.Counter{ID: 3, Name: "Loket 2"},
		ServingTicketID: sql.NullInt64{Int64: 40, Valid: true},
	})
	assert.Nil(t, idOnly.Serving)

	numOnly := dto.NewCounterResponse(model.CounterWithServing{
		Counter:       model.Counter{ID: 3, Name: "Loket 2"},
		ServingNumber: sql.NullInt64{Int64: 12, Valid: true},
	})
	assert.Nil(t, numOnly.Serving)
	assert.False(t, numOnly.IsDefault)
}

func TestNewCounterFromModel_NeverHasServing(t *testing.T) {
	got := dto.NewCounterFromModel(model.Counter{ID: 4, Name: "Baru", IsDefault: false})
	assert.Equal(t, int64(4), got.ID)
	assert.Equal(t, "Baru", got.Name)
	assert.Nil(t, got.Serving)
}

func TestNewNoteResponse_NullNoteWhenNotSaved(t *testing.T) {
	got := dto.NewNoteResponse(model.TicketNote{TicketID: 8})
	assert.Equal(t, int64(8), got.TicketID)
	assert.Nil(t, got.Note)
	assert.Nil(t, got.CreatedAt)
	assert.Nil(t, got.UpdatedAt)
}

func TestNewNoteResponse_CopiesNoteAndTimes(t *testing.T) {
	created := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	got := dto.NewNoteResponse(model.TicketNote{
		TicketID:  8,
		Note:      sql.NullString{String: "catatan", Valid: true},
		CreatedAt: sql.NullTime{Time: created, Valid: true},
		UpdatedAt: sql.NullTime{Time: updated, Valid: true},
	})
	if assert.NotNil(t, got.Note) {
		assert.Equal(t, "catatan", *got.Note)
	}
	if assert.NotNil(t, got.CreatedAt) {
		assert.True(t, created.Equal(*got.CreatedAt))
	}
	if assert.NotNil(t, got.UpdatedAt) {
		assert.True(t, updated.Equal(*got.UpdatedAt))
	}
}
