package handler_test

import (
	"database/sql"
	"github.com/gin-gonic/gin"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func specTicketEngine(svc *specTicketSvc, userID int64) *gin.Engine {
	e := specEngine(userID)
	h := handler.NewTicketHandler(svc)
	e.POST("/queues/:queue_id/join", h.Join)
	e.POST("/queues/:queue_id/next", h.Next)
	e.GET("/queues/:queue_id/tickets", h.List)
	e.GET("/queues/:queue_id/status", h.Status)
	e.GET("/users/me/tickets", h.MyTickets)
	return e
}

func TestSpec_Join_ReturnsAssignedNumberWithNullTimestamps(t *testing.T) {
	// 4.2 langkah 6 / US-14: konfirmasi memuat nomor yang diberikan; timestamp kosong = null.
	created := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	svc := &specTicketSvc{ticket: model.Ticket{
		ID: 9, QueueID: 1, TicketNumber: 1, SortKey: 65536, UserID: 42,
		Status: "waiting", CreatedAt: created,
	}}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodPost, "/queues/1/join", "")

	require.Equal(t, http.StatusOK, w.Code)
	ticket := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, float64(1), ticket["ticket_number"])
	assert.Equal(t, "waiting", ticket["status"])
	assert.Equal(t, float64(42), ticket["user_id"])
	for _, key := range []string{"last_notified_at", "completed_at", "cancelled_at"} {
		assert.Contains(t, ticket, key)
		assert.Nil(t, ticket[key], key)
	}
	assert.Equal(t, int64(42), svc.gotUser)
	assert.Equal(t, int64(1), svc.gotQueue)
}

func TestSpec_Join_QueueClosedIs409WithoutTicket(t *testing.T) {
	// 4.2 edge case: antrian tertutup -> pesan "queue is currently closed", tidak ada nomor.
	svc := &specTicketSvc{err: apperror.ErrQueueClosed}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodPost, "/queues/1/join", "")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "QUEUE_CLOSED", specErrorCode(t, w))
	assert.NotContains(t, w.Body.String(), `"ticket_number"`)
}

func TestSpec_Join_UnknownQueueIs404(t *testing.T) {
	// TC-05-07: antrian tidak ada -> 404.
	svc := &specTicketSvc{err: apperror.ErrNotFound}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodPost, "/queues/1/join", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "NOT_FOUND", specErrorCode(t, w))
}

func TestSpec_Next_EmptyQueueIs409QueueEmpty(t *testing.T) {
	// TC-03-01: Next pada antrian kosong -> 409 QUEUE_EMPTY, nomor tidak berubah.
	svc := &specTicketSvc{err: apperror.ErrQueueEmpty}
	e := specTicketEngine(svc, 7)

	w := specRequest(e, http.MethodPost, "/queues/1/next", "")

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Equal(t, "QUEUE_EMPTY", specErrorCode(t, w))
}

func TestSpec_Next_CustomerIs404(t *testing.T) {
	// TC-08-02: customer memanggil NEXT -> API menolak dengan 404.
	svc := &specTicketSvc{err: apperror.ErrNotFound}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodPost, "/queues/1/next", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "NOT_FOUND", specErrorCode(t, w))
}

func TestSpec_Next_ReturnsServedTicketNumber(t *testing.T) {
	// TC-03-02: respons Next memuat nomor yang dilayani (A-001 -> ticket_number 1).
	svc := &specTicketSvc{ticket: model.Ticket{ID: 1, QueueID: 1, TicketNumber: 1, UserID: 101, Status: "completed",
		CompletedAt: sql.NullTime{Time: time.Now(), Valid: true}}}
	e := specTicketEngine(svc, 7)

	w := specRequest(e, http.MethodPost, "/queues/1/next", "")

	require.Equal(t, http.StatusOK, w.Code)
	ticket := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, float64(1), ticket["ticket_number"])
	assert.Equal(t, "completed", ticket["status"])
	assert.NotNil(t, ticket["completed_at"])
	assert.Nil(t, ticket["cancelled_at"])
}

func TestSpec_Status_HolderSeesServingAndAheadCount(t *testing.T) {
	// TC-06-01 / 4.4: nomor sendiri, "sedang dilayani" null bila belum ada, dan orang di depan.
	two := 2
	svc := &specTicketSvc{status: model.QueueStatus{
		Queue:        model.Queue{ID: 1, QueueName: "Loket Utama", Status: "active", CreatedAt: time.Now()},
		MyTicket:     &model.Ticket{ID: 3, QueueID: 1, TicketNumber: 3, UserID: 42, Status: "waiting"},
		WaitingAhead: &two,
		NextNumber:   func() *int { n := 1; return &n }(),
	}}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodGet, "/queues/1/status", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "queue_status", data["action"])
	assert.Nil(t, data["current_number"])
	assert.Equal(t, float64(2), data["waiting_ahead"])
	assert.Nil(t, data["previous_number"])
	assert.Equal(t, float64(1), data["next_number"])
	my := data["my_ticket"].(map[string]interface{})
	assert.Equal(t, float64(3), my["ticket_number"])
	assert.Equal(t, "waiting", my["status"])
	queue := data["queue"].(map[string]interface{})
	assert.NotContains(t, queue, "owner_id")
}

func TestSpec_Status_NonHolderIs404(t *testing.T) {
	// TC-08-01: pengguna tanpa tiket -> 404; respons tidak memuat data antrian.
	svc := &specTicketSvc{err: apperror.ErrNotFound}
	e := specTicketEngine(svc, 99)

	w := specRequest(e, http.MethodGet, "/queues/1/status", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), `"my_ticket"`)
}

func TestSpec_List_OwnerStatusFilterIsForwarded(t *testing.T) {
	// US-6 / 4.1: daftar pelanggan dengan filter status; filter diteruskan ke service.
	svc := &specTicketSvc{tickets: []model.Ticket{{ID: 1, TicketNumber: 1, Status: "waiting"}}}
	e := specTicketEngine(svc, 7)

	w := specRequest(e, http.MethodGet, "/queues/1/tickets?status=waiting,skipped", "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"waiting", "skipped"}, svc.gotStatus)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Len(t, data["tickets"], 1)
}

func TestSpec_List_NoCustomersIsEmptyArray(t *testing.T) {
	// US-6: antrian tanpa pelanggan -> daftar kosong berupa [].
	svc := &specTicketSvc{tickets: []model.Ticket{}}
	e := specTicketEngine(svc, 7)

	w := specRequest(e, http.MethodGet, "/queues/1/tickets", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, []interface{}{}, data["tickets"])
}

func TestSpec_MyTickets_ReturnsOwnTicketsWithQueueInfo(t *testing.T) {
	// 4.4 / US-15: "tiket saya" memuat info antrian; tiket tetap bisa dibuka ulang tanpa join ulang.
	svc := &specTicketSvc{}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodGet, "/users/me/tickets", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "list_my_tickets", data["action"])
	assert.Equal(t, []interface{}{}, data["tickets"])
	assert.Equal(t, int64(42), svc.gotUser)
}
