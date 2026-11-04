package handler_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"qms/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type stubHistoryService struct {
	page   service.HistoryQueuesPage
	myPage service.MyHistoryPage
	detail service.HistoryQueueDetail
	err    error

	gotOwner  int64
	gotQueue  int64
	gotCursor string
	gotLimit  int

	exportQueue   model.HistoryQueue
	exportTickets []model.HistoryTicket
	exportErr     error
	streamErr     error
}

func (s *stubHistoryService) ListQueues(_ context.Context, ownerID int64, cursor string, limit int) (service.HistoryQueuesPage, error) {
	s.gotOwner, s.gotCursor, s.gotLimit = ownerID, cursor, limit
	return s.page, s.err
}

func (s *stubHistoryService) ListMyHistory(_ context.Context, userID int64, cursor string, limit int) (service.MyHistoryPage, error) {
	s.gotOwner, s.gotCursor, s.gotLimit = userID, cursor, limit
	return s.myPage, s.err
}

func (s *stubHistoryService) ExportQueue(_ context.Context, ownerID, queueID int64) (model.HistoryQueue, error) {
	s.gotOwner, s.gotQueue = ownerID, queueID
	if s.exportErr != nil {
		return model.HistoryQueue{}, s.exportErr
	}
	return s.exportQueue, nil
}

func (s *stubHistoryService) StreamTickets(_ context.Context, _ int64, sink service.TicketSink) error {
	if s.streamErr != nil {
		return s.streamErr
	}
	return sink.WriteTickets(s.exportTickets)
}

func (s *stubHistoryService) GetQueue(_ context.Context, ownerID, queueID int64, cursor string, limit int) (service.HistoryQueueDetail, error) {
	s.gotOwner, s.gotQueue, s.gotCursor, s.gotLimit = ownerID, queueID, cursor, limit
	return s.detail, s.err
}

func newHistoryEngine(svc *stubHistoryService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewHistoryHandler(svc)
	e := gin.New()
	e.Use(func(c *gin.Context) { c.Set("user_id", int64(7)); c.Next() })
	e.GET("/history/queues", h.ListQueues)
	e.GET("/history/queues/:queue_id", h.GetQueue)
	e.GET("/history/queues/:queue_id/export", h.Export)
	e.GET("/users/me/history", h.ListMine)
	return e
}

func TestHistoryList_ForwardsOwnerCursorAndLimit(t *testing.T) {
	svc := &stubHistoryService{}
	w := doGet(newHistoryEngine(svc), "/history/queues?cursor=123.4&limit=15")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7), svc.gotOwner)
	assert.Equal(t, "123.4", svc.gotCursor)
	assert.Equal(t, 15, svc.gotLimit)
}

func TestHistoryList_EmptyBodyShape(t *testing.T) {
	w := doGet(newHistoryEngine(&stubHistoryService{}), "/history/queues")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"queues":[]`, "daftar kosong adalah array, bukan null")
	assert.Contains(t, w.Body.String(), `"next_cursor":null`)
}

func TestHistoryList_BadLimit(t *testing.T) {
	for _, target := range []string{"/history/queues?limit=abc", "/history/queues?limit=-5"} {
		w := doGet(newHistoryEngine(&stubHistoryService{}), target)
		assert.Equal(t, http.StatusBadRequest, w.Code, target)
	}
}

func TestHistory_ErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not_found", apperror.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"invalid_cursor", apperror.ErrInvalidCursor, http.StatusBadRequest, "BAD_REQUEST"},
		{"internal", errors.New("db down"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, c := range cases {
		for _, target := range []string{"/history/queues", "/history/queues/5"} {
			w := doGet(newHistoryEngine(&stubHistoryService{err: c.err}), target)
			assert.Equal(t, c.status, w.Code, c.name+" "+target)
			assert.Contains(t, w.Body.String(), `"`+c.code+`"`)
		}
	}
}

func TestHistoryDetail_BadQueueID(t *testing.T) {
	w := doGet(newHistoryEngine(&stubHistoryService{}), "/history/queues/abc")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHistoryDetail_ForwardsQueueAndBody(t *testing.T) {
	svc := &stubHistoryService{detail: service.HistoryQueueDetail{
		Queue:      model.HistoryQueue{ID: 5, QueueName: "Loket"},
		NextCursor: "50",
	}}
	w := doGet(newHistoryEngine(svc), "/history/queues/5?cursor=10&limit=50")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(5), svc.gotQueue)
	assert.Equal(t, "10", svc.gotCursor)
	assert.Contains(t, w.Body.String(), `"tickets":[]`)
	assert.Contains(t, w.Body.String(), `"next_cursor":"50"`)
}

func exportStub() *stubHistoryService {
	return &stubHistoryService{
		exportQueue:   model.HistoryQueue{ID: 5, QueueName: "Loket Utama", TicketPrefix: "A"},
		exportTickets: []model.HistoryTicket{{TicketID: 1, TicketNumber: 1, Status: "completed", CustomerName: "budi", CustomerEmail: "b@x"}},
	}
}

func TestExport_CSVHeadersAndBody(t *testing.T) {
	svc := exportStub()
	w := doGet(newHistoryEngine(svc), "/history/queues/5/export?format=csv")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="riwayat-loket-utama-5.csv"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.True(t, strings.HasPrefix(w.Body.String(), "\xEF\xBB\xBF"), "BOM UTF-8")
	assert.Contains(t, w.Body.String(), "A-001,budi,b@x,Selesai")
	assert.Equal(t, int64(7), svc.gotOwner)
}

func TestExport_XLSXHeaders(t *testing.T) {
	w := doGet(newHistoryEngine(exportStub()), "/history/queues/5/export?format=xlsx")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="riwayat-loket-utama-5.xlsx"`, w.Header().Get("Content-Disposition"))
	assert.True(t, strings.HasPrefix(w.Body.String(), "PK"))
}

func TestExport_BadRequests(t *testing.T) {
	for _, target := range []string{
		"/history/queues/5/export",
		"/history/queues/5/export?format=",
		"/history/queues/5/export?format=pdf",
		"/history/queues/5/export?format=CSV",
		"/history/queues/abc/export?format=csv",
	} {
		w := doGet(newHistoryEngine(exportStub()), target)
		assert.Equal(t, http.StatusBadRequest, w.Code, target)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json", target)
		assert.Empty(t, w.Header().Get("Content-Disposition"), target)
	}
}

func TestExport_NotFoundIsJSONWithoutDownloadHeaders(t *testing.T) {
	svc := exportStub()
	svc.exportErr = apperror.ErrNotFound
	w := doGet(newHistoryEngine(svc), "/history/queues/5/export?format=csv")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"NOT_FOUND"`)
	assert.Empty(t, w.Header().Get("Content-Disposition"))
}

func TestExport_FailureBeforeFirstByteIsJSON500(t *testing.T) {
	svc := exportStub()
	svc.streamErr = errors.New("db down")
	w := doGet(newHistoryEngine(svc), "/history/queues/5/export?format=xlsx")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, w.Body.String(), `"INTERNAL_ERROR"`)
	assert.Empty(t, w.Header().Get("Content-Disposition"), "tidak ada header unduhan untuk respons galat")
}

func TestMyHistory_ForwardsTokenUserCursorAndLimit(t *testing.T) {
	svc := &stubHistoryService{}
	w := doGet(newHistoryEngine(svc), "/users/me/history?cursor=123.4&limit=15")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(7), svc.gotOwner, "user dari token, bukan dari parameter")
	assert.Equal(t, "123.4", svc.gotCursor)
	assert.Equal(t, 15, svc.gotLimit)
}

func TestMyHistory_EmptyBodyShapeAndNoPrivateFields(t *testing.T) {
	w := doGet(newHistoryEngine(&stubHistoryService{}), "/users/me/history")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"tickets":[]`)
	assert.Contains(t, w.Body.String(), `"next_cursor":null`)

	withRow := &stubHistoryService{myPage: service.MyHistoryPage{Tickets: []model.UserHistoryTicket{{TicketID: 1, QueueName: "Loket", Status: "completed"}}}}
	body := doGet(newHistoryEngine(withRow), "/users/me/history").Body.String()
	assert.NotContains(t, body, `"note"`, "catatan owner tidak pernah dikirim ke customer")
	assert.NotContains(t, body, "customer_email")
}

func TestMyHistory_BadLimitAndErrorMapping(t *testing.T) {
	for _, target := range []string{"/users/me/history?limit=abc", "/users/me/history?limit=-1"} {
		assert.Equal(t, http.StatusBadRequest, doGet(newHistoryEngine(&stubHistoryService{}), target).Code, target)
	}
	assert.Equal(t, http.StatusBadRequest, doGet(newHistoryEngine(&stubHistoryService{err: apperror.ErrInvalidCursor}), "/users/me/history").Code)
	assert.Equal(t, http.StatusInternalServerError, doGet(newHistoryEngine(&stubHistoryService{err: errors.New("db down")}), "/users/me/history").Code)
}
