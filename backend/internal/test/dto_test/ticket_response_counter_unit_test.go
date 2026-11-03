package dto_test

import (
	"database/sql"
	"qms/internal/dto"
	"qms/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Respons tiket membawa counter dan waktu panggil supaya papan owner bisa menampilkan nomor per counter.
func TestNewTicketResponse_CarriesCounterAndCalledAt(t *testing.T) {
	called := time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	got := dto.NewTicketResponse(model.Ticket{
		ID:              7,
		Status:          "currently_served",
		CalledCounterID: 2,
		CalledAt:        sql.NullTime{Time: called, Valid: true},
	})

	if assert.NotNil(t, got.CalledCounterID) {
		assert.Equal(t, int64(2), *got.CalledCounterID)
	}
	if assert.NotNil(t, got.CalledAt) {
		assert.True(t, got.CalledAt.Equal(called))
	}
}

// Tiket yang belum dipanggil: counter dan waktu panggil null (bukan 0 atau waktu kosong).
func TestNewTicketResponse_WaitingTicketHasNullCounter(t *testing.T) {
	got := dto.NewTicketResponse(model.Ticket{ID: 8, Status: "waiting"})

	assert.Nil(t, got.CalledCounterID)
	assert.Nil(t, got.CalledAt)
}
