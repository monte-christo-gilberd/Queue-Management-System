package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"strconv"

	"github.com/gin-gonic/gin"
)

type CounterService interface {
	List(
		ctx context.Context,
		queueID,
		requesterID int64,
	) ([]model.CounterWithServing, error)
	Create(
		ctx context.Context,
		queueID,
		requesterID int64,
		name string,
	) (model.Counter, error)
	Rename(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		name string,
	) (model.Counter, error)
	Delete(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
	) error
}

type CounterHandler struct {
	service CounterService
}

func NewCounterHandler(
	service CounterService,
) *CounterHandler {
	return &CounterHandler{
		service: service,
	}
}

type counterNameRequest struct {
	Name string `json:"name"`
}

// List: GET /api/v1/queues/:queue_id/counters — daftar counter beserta nomor yang sedang dilayani.
func (h *CounterHandler) List(c *gin.Context) {
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	counters, err := h.service.List(c.Request.Context(), queueID, c.GetInt64("user_id"))
	if err != nil {
		respondCounterError(c, err)
		return
	}
	resp := make([]dto.CounterResponse, 0, len(counters))
	for _, counter := range counters {
		resp = append(resp, dto.NewCounterResponse(counter))
	}
	respondData(c, http.StatusOK, gin.H{"action": "list_counters", "queue_id": queueID, "counters": resp})
}

// Create: POST /api/v1/queues/:queue_id/counters — buat counter. Nama kosong = nama otomatis "Counter N".
func (h *CounterHandler) Create(c *gin.Context) {
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	var req counterNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	counter, err := h.service.Create(c.Request.Context(), queueID, c.GetInt64("user_id"), req.Name)
	if err != nil {
		respondCounterError(c, err)
		return
	}
	respondData(c, http.StatusCreated, dto.NewCounterFromModel(counter))
}

// Rename: PUT /api/v1/queues/:queue_id/counters/:counter_id — ganti nama (counter default boleh).
func (h *CounterHandler) Rename(c *gin.Context) {
	queueID, counterID, ok := parseQueueAndCounterID(c)
	if !ok {
		return
	}
	var req counterNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	counter, err := h.service.Rename(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID, req.Name)
	if err != nil {
		respondCounterError(c, err)
		return
	}
	respondData(c, http.StatusOK, dto.NewCounterFromModel(counter))
}

// Delete: DELETE /api/v1/queues/:queue_id/counters/:counter_id — hapus counter. Default: 409.
func (h *CounterHandler) Delete(c *gin.Context) {
	queueID, counterID, ok := parseQueueAndCounterID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID); err != nil {
		respondCounterError(c, err)
		return
	}
	respondData(c, http.StatusOK, gin.H{"action": "delete_counter", "queue_id": queueID, "counter_id": counterID})
}

func parseQueueID(c *gin.Context) (int64, bool) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return 0, false
	}
	return queueID, true
}

func parseQueueAndCounterID(c *gin.Context) (int64, int64, bool) {
	queueID, ok := parseQueueID(c)
	if !ok {
		return 0, 0, false
	}
	counterID, err := strconv.ParseInt(c.Param("counter_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid counter id")
		return 0, 0, false
	}
	return queueID, counterID, true
}

func respondCounterError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or counter not found")
	case errors.Is(err, apperror.ErrQueueClosed):
		respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
	case errors.Is(err, apperror.ErrDuplicateCounterName):
		respondError(c, http.StatusConflict, "DUPLICATE_COUNTER_NAME", "a counter with this name already exists")
	case errors.Is(err, apperror.ErrCounterLimit):
		respondError(c, http.StatusConflict, "COUNTER_LIMIT", "a queue can have at most 10 counters")
	case errors.Is(err, apperror.ErrDefaultCounter):
		respondError(c, http.StatusConflict, "DEFAULT_COUNTER", "the default counter cannot be deleted")
	case errors.Is(err, apperror.ErrInvalidCounterName):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "counter name must be 1-30 characters")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
