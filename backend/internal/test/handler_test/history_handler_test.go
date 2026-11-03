package handler_test

import (
	"context"
	"errors"
	"net/http"
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
	detail service.HistoryQueueDetail
	err    error

	gotOwner  int64
	gotQueue  int64
	gotCursor string
	gotLimit  int
}

func (s *stubHistoryService) ListQueues(_ context.Context, ownerID int64, cursor string, limit int) (service.HistoryQueuesPage, error) {
	s.gotOwner, s.gotCursor, s.gotLimit = ownerID, cursor, limit
	return s.page, s.err
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
