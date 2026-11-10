package dto_test

import (
	"database/sql"
	"encoding/json"
	"qms/internal/dto"
	"qms/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func specMarshalMap(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func TestSpec_QueueResponse_OpenQueueHasNullClosedAtAndDesc(t *testing.T) {
	// Kontrak API: field kosong = null; antrian aktif belum punya closed_at.
	q := model.Queue{ID: 1, OwnerID: 7, QueueName: "Loket", Status: "active", CreatedAt: time.Now()}

	got := specMarshalMap(t, dto.NewQueueResponse(q))

	assert.Nil(t, got["queue_desc"])
	assert.Nil(t, got["closed_at"])
	assert.Equal(t, "active", got["status"])
	for _, key := range []string{"id", "owner_id", "queue_name", "queue_desc", "status", "created_at", "closed_at"} {
		assert.Contains(t, got, key)
	}
}

func TestSpec_QueueResponse_ClosedQueueCarriesClosedAt(t *testing.T) {
	// 4.1 langkah 5 / 4.15: antrian yang ditutup berstatus closed dan punya waktu tutup.
	closedAt := time.Date(2026, 10, 17, 18, 0, 0, 0, time.UTC)
	q := model.Queue{
		ID: 1, OwnerID: 7, QueueName: "Loket", QueueDesc: sql.NullString{String: "Sore", Valid: true},
		Status: "closed", CreatedAt: closedAt.Add(-time.Hour), ClosedAt: sql.NullTime{Time: closedAt, Valid: true},
	}

	got := specMarshalMap(t, dto.NewQueueResponse(q))

	assert.Equal(t, "closed", got["status"])
	assert.Equal(t, "2026-10-17T18:00:00Z", got["closed_at"])
	assert.Equal(t, "Sore", got["queue_desc"])
}

func TestSpec_PublicQueueResponse_KeepsOwnerIDOut(t *testing.T) {
	// 4.4 / TC-08-03: tampilan publik (pemegang tiket / join) tidak memuat owner_id.
	q := model.Queue{ID: 1, OwnerID: 7, QueueName: "Loket", Status: "active", CreatedAt: time.Now()}

	got := specMarshalMap(t, dto.NewPublicQueueResponse(q))

	assert.NotContains(t, got, "owner_id")
	assert.Equal(t, "Loket", got["queue_name"])
}

func TestSpec_QueueStatusResponse_NothingServedAndNoTicketAreNull(t *testing.T) {
	// TC-06-01 / 4.4: sebelum ada yang dilayani, "sedang dilayani" null; tanpa tiket, my_ticket null.
	s := model.QueueStatus{Queue: model.Queue{ID: 1, QueueName: "Loket", Status: "active", CreatedAt: time.Now()}}

	got := specMarshalMap(t, dto.NewQueueStatusResponse(s))

	for _, key := range []string{"my_ticket", "current_number", "waiting_ahead", "previous_number", "next_number"} {
		assert.Contains(t, got, key)
		assert.Nil(t, got[key], key)
	}
	assert.NotContains(t, got["queue"], "owner_id")
}

func TestSpec_QueueStatusResponse_CarriesCustomerNumbers(t *testing.T) {
	// TC-06-01: nomor sendiri, nomor dilayani, dan jumlah orang di depan tampil apa adanya.
	cur, ahead, mine := 1, 2, 3
	s := model.QueueStatus{
		Queue:         model.Queue{ID: 1, QueueName: "Loket", Status: "active", CreatedAt: time.Now()},
		MyTicket:      &model.Ticket{ID: 9, QueueID: 1, TicketNumber: mine, Status: "waiting", CreatedAt: time.Now()},
		CurrentNumber: &cur,
		WaitingAhead:  &ahead,
	}

	got := specMarshalMap(t, dto.NewQueueStatusResponse(s))

	assert.Equal(t, float64(1), got["current_number"])
	assert.Equal(t, float64(2), got["waiting_ahead"])
	my := got["my_ticket"].(map[string]interface{})
	assert.Equal(t, float64(3), my["ticket_number"])
	assert.Equal(t, "waiting", my["status"])
}

func TestSpec_UserTicketResponses_FlattenTicketAndQueueInfo(t *testing.T) {
	// 4.4 / US-15: daftar tiket saya memuat info antrian di samping data tiket.
	cur := 4
	list := []model.UserTicket{{
		Ticket:        model.Ticket{ID: 2, QueueID: 1, TicketNumber: 5, UserID: 42, Status: "waiting", CreatedAt: time.Now()},
		QueueName:     "Loket",
		QueueStatus:   "active",
		CurrentNumber: &cur,
	}}

	raw, err := json.Marshal(dto.NewUserTicketResponses(list))
	require.NoError(t, err)
	var got []map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))

	require.Len(t, got, 1)
	assert.Equal(t, "Loket", got[0]["queue_name"])
	assert.Equal(t, float64(5), got[0]["ticket_number"])
	assert.Equal(t, float64(4), got[0]["current_number"])
}

func TestSpec_UserTicketResponses_EmptyIsJSONArray(t *testing.T) {
	raw, err := json.Marshal(dto.NewUserTicketResponses(nil))
	require.NoError(t, err)
	assert.Equal(t, "[]", string(raw))
}

func TestSpec_MeResponse_ExposesIsOwnerFlag(t *testing.T) {
	// 4.18: profil menyatakan apakah user adalah owner.
	got := specMarshalMap(t, dto.MeResponse{ID: 1, Email: "a@b.com", Username: "a", IsOwner: true})

	assert.Equal(t, true, got["is_owner"])
	assert.Equal(t, "a", got["username"])
}

func TestNormalizeTicketPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"  ", "", true},
		{"a", "A", true},
		{" vip ", "VIP", true},
		{"B2", "B2", true},
		{"ABCD", "", false},
		{"A-", "", false},
		{"a b", "", false},
		{"<b>", "", false},
		{"é", "", false},
	}
	for _, c := range cases {
		got, ok := dto.NormalizeTicketPrefix(c.in)
		if ok != c.ok || got != c.want {
			t.Fatalf("NormalizeTicketPrefix(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
