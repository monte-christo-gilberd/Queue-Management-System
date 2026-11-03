package handler_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// stubCounterService: err dipakai untuk semua operasi.
type stubCounterService struct {
	err error
}

func (s *stubCounterService) List(context.Context, int64, int64) ([]model.CounterWithServing, error) {
	return []model.CounterWithServing{}, s.err
}

func (s *stubCounterService) Create(context.Context, int64, int64, string) (model.Counter, error) {
	return model.Counter{ID: 2, Name: "Counter 2"}, s.err
}

func (s *stubCounterService) Rename(context.Context, int64, int64, int64, string) (model.Counter, error) {
	return model.Counter{ID: 2, Name: "Loket"}, s.err
}

func (s *stubCounterService) Delete(context.Context, int64, int64, int64) error {
	return s.err
}

func newCounterEngine(svc *stubCounterService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := handler.NewCounterHandler(svc)
	e.GET("/queues/:queue_id/counters", h.List)
	e.POST("/queues/:queue_id/counters", h.Create)
	e.PUT("/queues/:queue_id/counters/:counter_id", h.Rename)
	e.DELETE("/queues/:queue_id/counters/:counter_id", h.Delete)
	return e
}

func doRequest(e *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(method, target, bytes.NewBufferString(body)))
	return w
}

func TestCounter_CreateOK(t *testing.T) {
	w := doRequest(newCounterEngine(&stubCounterService{}), http.MethodPost, "/queues/5/counters", `{}`)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"Counter 2"`)
}

func TestCounter_ErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"not_found", apperror.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"queue_closed", apperror.ErrQueueClosed, http.StatusConflict, "QUEUE_CLOSED"},
		{"duplicate", apperror.ErrDuplicateCounterName, http.StatusConflict, "DUPLICATE_COUNTER_NAME"},
		{"limit", apperror.ErrCounterLimit, http.StatusConflict, "COUNTER_LIMIT"},
		{"default", apperror.ErrDefaultCounter, http.StatusConflict, "DEFAULT_COUNTER"},
		{"invalid_name", apperror.ErrInvalidCounterName, http.StatusBadRequest, "BAD_REQUEST"},
		{"internal", errors.New("db down"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, c := range cases {
		e := newCounterEngine(&stubCounterService{err: c.err})

		w := doRequest(e, http.MethodPost, "/queues/5/counters", `{"name":"x"}`)
		assert.Equal(t, c.status, w.Code, c.name)
		assert.Contains(t, w.Body.String(), `"`+c.code+`"`, c.name)

		w = doRequest(e, http.MethodDelete, "/queues/5/counters/2", "")
		assert.Equal(t, c.status, w.Code, c.name+" (delete)")
	}
}

func TestCounter_RenameAndDeleteOK(t *testing.T) {
	e := newCounterEngine(&stubCounterService{})

	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodPut, "/queues/5/counters/2", `{"name":"Loket"}`).Code)
	assert.Equal(t, http.StatusOK, doRequest(e, http.MethodDelete, "/queues/5/counters/2", "").Code)
}

func TestCounter_BadIDsAndBody(t *testing.T) {
	e := newCounterEngine(&stubCounterService{})

	assert.Equal(t, http.StatusBadRequest, doRequest(e, http.MethodGet, "/queues/abc/counters", "").Code)
	assert.Equal(t, http.StatusBadRequest, doRequest(e, http.MethodPut, "/queues/5/counters/xyz", `{"name":"a"}`).Code)
	assert.Equal(t, http.StatusBadRequest, doRequest(e, http.MethodPost, "/queues/5/counters", `{"name":`).Code)
}
