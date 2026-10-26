package dto_test

import (
	"database/sql"
	"encoding/json"
	"qms/internal/dto"
	"qms/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTicketResponse_JSONShape(t *testing.T) {
	done := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tk := model.Ticket{
		ID: 1, QueueID: 2, TicketNumber: 3, SortKey: 65536, UserID: 4,
		Status:      "completed",
		CreatedAt:   done,
		CompletedAt: sql.NullTime{Time: done, Valid: true},
	}

	raw, err := json.Marshal(dto.NewTicketResponse(tk))
	assert.NoError(t, err)

	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, float64(3), got["ticket_number"])
	assert.Equal(t, "2026-01-02T03:04:05Z", got["completed_at"])
	assert.Nil(t, got["cancelled_at"])
	assert.Contains(t, got, "last_notified_at")
	assert.NotContains(t, got, "ID")
}

func TestTicketResponses_EmptyIsJSONArray(t *testing.T) {
	raw, err := json.Marshal(dto.NewTicketResponses(nil))
	assert.NoError(t, err)
	assert.JSONEq(t, `[]`, string(raw))
}

func TestQueueResponse_NullDescAndClosedAt(t *testing.T) {
	q := model.Queue{
		ID: 1, OwnerID: 2, QueueName: "A", Status: "active",
		QueueDesc: sql.NullString{String: "x", Valid: false},
	}

	raw, err := json.Marshal(dto.NewQueueResponse(q))
	assert.NoError(t, err)

	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "A", got["queue_name"])
	assert.Nil(t, got["queue_desc"])
	assert.Nil(t, got["closed_at"])
}

func TestTicketEvent_OmitsUserIDAndSortKey(t *testing.T) {
	raw, err := json.Marshal(dto.NewTicketEvent(model.Ticket{ID: 1, QueueID: 2, TicketNumber: 3, SortKey: 65536, UserID: 99, Status: "waiting"}))
	assert.NoError(t, err)

	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, float64(3), got["ticket_number"])
	assert.NotContains(t, got, "user_id")
	assert.NotContains(t, got, "sort_key")
}

func TestPublicQueueResponse_OmitsOwnerID(t *testing.T) {
	raw, err := json.Marshal(dto.NewPublicQueueResponse(model.Queue{ID: 1, OwnerID: 7, QueueName: "A", Status: "active"}))
	assert.NoError(t, err)

	var got map[string]interface{}
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "A", got["queue_name"])
	assert.NotContains(t, got, "owner_id")
}
