package dto_test

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"qms/internal/dto"
	"qms/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	out := map[string]any{}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// Tes kontrak: semua field yang dijanjikan di dokumen API harus ada di JSON, termasuk yang bernilai null.
func TestHistoryQueueResponse_ContractKeys(t *testing.T) {
	got := asMap(t, dto.NewHistoryQueueResponse(model.HistoryQueue{ID: 1}))
	for _, key := range []string{
		"id", "queue_name", "ticket_prefix", "opened_at", "closed_at",
		"total", "completed", "skipped", "cancelled", "deleted", "unserved",
	} {
		assert.Contains(t, got, key)
	}
}

func TestHistoryTicketResponse_ContractKeysAndNulls(t *testing.T) {
	got := asMap(t, dto.NewHistoryTicketResponse(model.HistoryTicket{TicketID: 1, Status: "waiting"}))
	for _, key := range []string{
		"ticket_id", "ticket_number", "outcome", "skip_count", "joined_at", "called_at", "completed_at",
		"cancelled_at", "customer_name", "customer_email", "counter_name", "note",
	} {
		assert.Contains(t, got, key)
	}
	assert.Nil(t, got["note"])
	assert.Nil(t, got["counter_name"])
	assert.Nil(t, got["called_at"])
}

func TestHistoryTicketResponse_FillsOptionalFields(t *testing.T) {
	now := time.Now()
	got := asMap(t, dto.NewHistoryTicketResponse(model.HistoryTicket{
		Status:      "completed",
		CalledAt:    sql.NullTime{Time: now, Valid: true},
		CompletedAt: sql.NullTime{Time: now, Valid: true},
		CounterName: sql.NullString{String: "Counter 2", Valid: true},
		Note:        sql.NullString{String: "minta struk", Valid: true},
	}))
	assert.Equal(t, "completed", got["outcome"])
	assert.Equal(t, "Counter 2", got["counter_name"])
	assert.Equal(t, "minta struk", got["note"])
	assert.NotNil(t, got["called_at"])
	assert.Nil(t, got["cancelled_at"])
}

func TestHistoryOutcome_UnservedCoversWaitingAndServing(t *testing.T) {
	assert.Equal(t, "unserved", model.HistoryOutcome("waiting"))
	assert.Equal(t, "unserved", model.HistoryOutcome("currently_served"))
	for _, s := range []string{"completed", "skipped", "cancelled", "deleted"} {
		assert.Equal(t, s, model.HistoryOutcome(s))
	}
}

func TestHistoryListResponses_CursorAndEmptyArrays(t *testing.T) {
	empty := asMap(t, dto.NewHistoryQueuesResponse(nil, ""))
	assert.Equal(t, []any{}, empty["queues"], "array kosong, bukan null")
	assert.Nil(t, empty["next_cursor"])
	assert.Equal(t, "list_history_queues", empty["action"])

	more := asMap(t, dto.NewHistoryQueueDetailResponse(model.HistoryQueue{ID: 2}, nil, "40"))
	assert.Equal(t, "40", more["next_cursor"])
	assert.Equal(t, []any{}, more["tickets"])
	assert.Equal(t, "history_queue", more["action"])
}
