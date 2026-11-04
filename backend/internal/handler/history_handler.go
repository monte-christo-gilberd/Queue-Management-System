package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/export"
	"qms/internal/model"
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
	ListMyHistory(
		ctx context.Context,
		userID int64,
		cursor string,
		limit int,
	) (service.MyHistoryPage, error)
	exportService
}

// exportService: bagian HistoryService untuk ekspor.
type exportService interface {
	ExportQueue(ctx context.Context, ownerID, queueID int64) (model.HistoryQueue, error)
	StreamTickets(ctx context.Context, queueID int64, sink service.TicketSink) error
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

// ListMine: GET /api/v1/users/me/history?cursor=&limit= — riwayat tiket milik user yang login (customer), satu
// baris per tiket, terbaru dulu. Tidak ada parameter user: hanya tiket milik token yang dikembalikan.
func (h *HistoryHandler) ListMine(c *gin.Context) {
	limit, ok := historyLimit(c)
	if !ok {
		return
	}
	page, err := h.service.ListMyHistory(c.Request.Context(), c.GetInt64("user_id"), c.Query("cursor"), limit)
	if err != nil {
		respondHistoryError(c, err)
		return
	}
	respondData(c, http.StatusOK, dto.NewMyHistoryResponse(page.Tickets, page.NextCursor))
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

// lazyDownload menunda header unduhan sampai byte pertama ditulis. Selama belum ada byte yang keluar, kegagalan
// (mis. database) masih bisa dibalas sebagai JSON 500 biasa, bukan berkas yang rusak.
type lazyDownload struct {
	c           *gin.Context
	contentType string
	filename    string
	started     bool
}

func (l *lazyDownload) Write(p []byte) (int, error) {
	if !l.started {
		l.started = true
		h := l.c.Writer.Header()
		h.Set("Content-Type", l.contentType)
		h.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, l.filename))
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		l.c.Status(http.StatusOK)
	}
	return l.c.Writer.Write(p)
}

// Export: GET /api/v1/history/queues/:queue_id/export?format=csv|xlsx — unduh seluruh nomor antrian yang sudah
// ditutup. Owner saja; bukan owner atau masih aktif: 404. Format wajib diisi; selain csv atau xlsx: 400.
func (h *HistoryHandler) Export(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	format, ok := export.ParseFormat(c.Query("format"))
	if !ok {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "format must be csv or xlsx")
		return
	}
	queue, err := h.service.ExportQueue(c.Request.Context(), c.GetInt64("user_id"), queueID)
	if err != nil {
		respondHistoryError(c, err)
		return
	}

	out := &lazyDownload{
		c:           c,
		contentType: format.ContentType(),
		filename:    export.FileName(queue.QueueName, queue.ID, format),
	}
	sink, err := export.NewSink(format, out, queue)
	if err == nil {
		err = h.service.StreamTickets(c.Request.Context(), queueID, sink)
	}
	if err == nil {
		err = sink.Close()
	}
	if err != nil {
		if !out.started {
			respondHistoryError(c, err)
			return
		}
		// Byte sudah terkirim: status tidak bisa diubah lagi. Catat dan hentikan; berkas terpotong.
		_ = c.Error(err)
		c.Abort()
	}
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
