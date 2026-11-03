package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/service"

	"github.com/gin-gonic/gin"
)

type HistoryService interface {
	ListQueues(
		ctx context.Context,
		ownerID int64,
		cursor string,
		limit int,
	) (service.HistoryQueuesPage, error)
	GetQueue(
		ctx context.Context,
		ownerID,
		queueID int64,
		cursor string,
		limit int,
	) (service.HistoryQueueDetail, error)
}

type HistoryHandler struct {
	service HistoryService
}

func NewHistoryHandler(service HistoryService) *HistoryHandler {
	return &HistoryHandler{service: service}
}

// ListQueues: GET /api/v1/history/queues?cursor=&limit= — antrian milik user yang sudah ditutup, terbaru dulu.
// User yang bukan owner mendapat daftar kosong.
func (h *HistoryHandler) ListQueues(c *gin.Context) {
	limit, ok := historyLimit(c)
	if !ok {
		return
	}
	page, err := h.service.ListQueues(c.Request.Context(), c.GetInt64("user_id"), c.Query("cursor"), limit)
	if err != nil {
		respondHistoryError(c, err)
		return
	}
	respondData(c, http.StatusOK, dto.NewHistoryQueuesResponse(page.Queues, page.NextCursor))
}

// GetQueue: GET /api/v1/history/queues/:queue_id?cursor=&limit= — ringkasan dan nomor satu antrian yang sudah
// ditutup. Bukan milik owner atau masih aktif: 404.
func (h *HistoryHandler) GetQueue(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	limit, ok := historyLimit(c)
	if !ok {
		return
	}
	detail, err := h.service.GetQueue(c.Request.Context(), c.GetInt64("user_id"), queueID, c.Query("cursor"), limit)
	if err != nil {
		respondHistoryError(c, err)
		return
	}
	respondData(c, http.StatusOK, dto.NewHistoryQueueDetailResponse(detail.Queue, detail.Tickets, detail.NextCursor))
}

// historyLimit membaca ?limit=. Kosong berarti 0 (service memakai bawaan); bukan angka: 400.
func historyLimit(c *gin.Context) (int, bool) {
	raw := c.Query("limit")
	if raw == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 0 {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid limit")
		return 0, false
	}
	return limit, true
}

func respondHistoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
	case errors.Is(err, apperror.ErrInvalidCursor):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid cursor")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
