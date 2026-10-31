package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type stubTicketService struct {
	userTickets []model.UserTicket
	status      model.QueueStatus
	tickets     []model.Ticket
	err         error
	joinErr     error
	nextErr     error
	statuses    []string
	called      bool
	token       string
	joinQueue   model.Queue
	resolveErr  error
}

func (s *stubTicketService) JoinQueue(context.Context, int64, int64) (model.Ticket, error) {
	return model.Ticket{}, s.joinErr
}

func (s *stubTicketService) ResolveJoinToken(_ context.Context, token string) (model.Queue, error) {
	s.token = token
	return s.joinQueue, s.resolveErr
}

func (s *stubTicketService) NextTicket(context.Context, int64, int64, int64) (model.Ticket, error) {
	return model.Ticket{}, s.nextErr
}

func (s *stubTicketService) RecallTicket(context.Context, int64, int64) (model.Ticket, error) {
	return model.Ticket{}, s.nextErr
}

func (s *stubTicketService) SkipTicket(context.Context, int64, int64) (model.Ticket, error) {
	return model.Ticket{}, s.nextErr
}

func (s *stubTicketService) ListMyTickets(
	_ context.Context,
	_ int64,
) ([]model.UserTicket, error) {
	return s.userTickets, s.err
}

func (s *stubTicketService) QueueStatus(
	_ context.Context,
	_, _ int64,
) (model.QueueStatus, error) {
	s.called = true
	return s.status, s.err
}

func (s *stubTicketService) ListTickets(
	_ context.Context,
	_, _ int64,
	statuses []string,
) ([]model.Ticket, error) {
	s.called = true
	s.statuses = statuses
	return s.tickets, s.err
}

func newListEngine(svc *stubTicketService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := handler.NewTicketHandlerWithQR(svc, svc)
	e.GET("/queues/:queue_id/tickets", h.List)
	e.GET("/queues/:queue_id/status", h.Status)
	e.POST("/join", h.Join)
	e.POST("/queues/:queue_id/next", h.Next)
	return e
}

func doGet(e *gin.Engine, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestListTickets_OK(t *testing.T) {
	svc := &stubTicketService{tickets: []model.Ticket{{ID: 1, QueueID: 5, TicketNumber: 1, Status: "waiting"}}}

	w := doGet(newListEngine(svc), "/queues/5/tickets?status=waiting,skipped&status=completed")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"action":"list_tickets","queue_id":5,"tickets":[{
		"id":1,"queue_id":5,"ticket_number":1,"sort_key":0,"user_id":0,"status":"waiting",
		"skip_count":0,"last_notified_at":null,"created_at":"0001-01-01T00:00:00Z",
		"completed_at":null,"cancelled_at":null,"called_counter_id":null,"called_counter_name":null,"called_at":null}]}}`, w.Body.String())
	assert.Equal(t, []string{"waiting", "skipped", "completed"}, svc.statuses)
}

func TestListTickets_EmptyIsEmptyArray(t *testing.T) {
	w := doGet(newListEngine(&stubTicketService{}), "/queues/5/tickets")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"action":"list_tickets","queue_id":5,"tickets":[]}}`, w.Body.String())
}

func TestListTickets_BadRequests(t *testing.T) {
	for _, target := range []string{"/queues/abc/tickets", "/queues/5/tickets?status=foo"} {
		svc := &stubTicketService{}
		w := doGet(newListEngine(svc), target)

		assert.Equal(t, http.StatusBadRequest, w.Code, target)
		assert.False(t, svc.called, target)
	}
}

func TestListTickets_NotFoundAndInternal(t *testing.T) {
	w := doGet(newListEngine(&stubTicketService{err: apperror.ErrNotFound}), "/queues/5/tickets")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"NOT_FOUND"`)

	w = doGet(newListEngine(&stubTicketService{err: assert.AnError}), "/queues/5/tickets")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func doPost(e *gin.Engine, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodPost, target, bytes.NewBufferString(`{"t":"token-abc"}`)))
	return w
}

func TestJoin_ErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		svc    *stubTicketService
		status int
		code   string
	}{
		{"qr_invalid", &stubTicketService{resolveErr: apperror.ErrQRInvalid}, http.StatusNotFound, "QR_INVALID"},
		{"qr_expired", &stubTicketService{resolveErr: apperror.ErrQRExpired}, http.StatusGone, "QR_EXPIRED"},
		{"qr_unavailable", &stubTicketService{resolveErr: apperror.ErrQRUnavailable}, http.StatusServiceUnavailable, "QR_UNAVAILABLE"},
		{"not_found", &stubTicketService{resolveErr: apperror.ErrNotFound}, http.StatusNotFound, "NOT_FOUND"},
		{"queue_closed", &stubTicketService{joinErr: apperror.ErrQueueClosed}, http.StatusConflict, "QUEUE_CLOSED"},
		{"already_joined", &stubTicketService{joinErr: apperror.ErrAlreadyJoined}, http.StatusConflict, "ALREADY_JOINED"},
		{"retry_exhausted", &stubTicketService{joinErr: apperror.ErrRetryExhausted}, http.StatusServiceUnavailable, "RETRY_EXHAUSTED"},
	}
	for _, c := range cases {
		w := doPost(newListEngine(c.svc), "/join")

		assert.Equal(t, c.status, w.Code, c.name)
		assert.Contains(t, w.Body.String(), `"`+c.code+`"`)
	}
}

func TestNext_ErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{apperror.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{apperror.ErrQueueEmpty, http.StatusConflict, "QUEUE_EMPTY"},
	}
	for _, c := range cases {
		w := doPost(newListEngine(&stubTicketService{nextErr: c.err}), "/queues/5/next")

		assert.Equal(t, c.status, w.Code, c.code)
		assert.Contains(t, w.Body.String(), `"`+c.code+`"`)
	}
}

func TestStatus_OK(t *testing.T) {
	current, ahead := 3, 1
	svc := &stubTicketService{status: model.QueueStatus{
		Queue:         model.Queue{ID: 5, OwnerID: 7, QueueName: "A", Status: "active"},
		MyTicket:      &model.Ticket{ID: 9, QueueID: 5, TicketNumber: 5, Status: "waiting"},
		CurrentNumber: &current,
		WaitingAhead:  &ahead,
	}}

	w := doGet(newListEngine(svc), "/queues/5/status")

	assert.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data map[string]interface{} `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "queue_status", got.Data["action"])
	assert.Equal(t, float64(3), got.Data["current_number"])
	assert.Equal(t, float64(1), got.Data["waiting_ahead"])
	assert.Equal(t, float64(5), got.Data["my_ticket"].(map[string]interface{})["ticket_number"])
	queue := got.Data["queue"].(map[string]interface{})
	assert.Equal(t, "A", queue["queue_name"])
	assert.NotContains(t, queue, "owner_id")
}

func TestStatus_NullsWhenNoTicketAndNothingServed(t *testing.T) {
	w := doGet(newListEngine(&stubTicketService{status: model.QueueStatus{Queue: model.Queue{ID: 5}}}), "/queues/5/status")

	assert.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data map[string]interface{} `json:"data"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	for _, k := range []string{"my_ticket", "current_number", "waiting_ahead"} {
		assert.Contains(t, got.Data, k)
		assert.Nil(t, got.Data[k], k)
	}
}

func TestStatus_ErrorsAndBadID(t *testing.T) {
	w := doGet(newListEngine(&stubTicketService{err: apperror.ErrNotFound}), "/queues/5/status")
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = doGet(newListEngine(&stubTicketService{err: assert.AnError}), "/queues/5/status")
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	svc := &stubTicketService{}
	w = doGet(newListEngine(svc), "/queues/abc/status")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, svc.called)
}

func TestMyTickets_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubTicketService{userTickets: []model.UserTicket{{
		Ticket:      model.Ticket{ID: 3, QueueID: 5, TicketNumber: 1, Status: "waiting"},
		QueueName:   "Loket A",
		QueueStatus: "active",
	}}}
	e := gin.New()
	e.GET("/users/me/tickets", handler.NewTicketHandler(svc).MyTickets)

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/me/tickets", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"action":"list_my_tickets","tickets":[{
		"id":3,"queue_id":5,"ticket_number":1,"ticket_prefix":"","sort_key":0,"user_id":0,"status":"waiting",
		"skip_count":0,"last_notified_at":null,"created_at":"0001-01-01T00:00:00Z",
		"completed_at":null,"cancelled_at":null,"called_counter_id":null,"called_counter_name":null,"called_at":null,"queue_name":"Loket A","queue_status":"active","current_number":null}]}}`, w.Body.String())
}
