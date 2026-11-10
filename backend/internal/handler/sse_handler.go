package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/sse"
	"strconv"

	"github.com/gin-gonic/gin"
)

// QueueValidator: owner dan pemegang tiket boleh subscribe; selain itu apperror.ErrNotFound.
type QueueValidator interface {
	// ViewQueue mengambil antrian bila requesterID boleh melihatnya; bool menandakan apakah requester adalah owner.
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, bool, error)
}

// SSEHandler menangani koneksi SSE per antrian.
type SSEHandler struct {
	// hub menyimpan semua koneksi SSE dan menyebarkan event.
	hub *sse.Hub
	// queues memeriksa hak akses user terhadap antrian. Nil berarti tidak ada pemeriksaan.
	queues QueueValidator
}

// NewSSEHandler membuat SSEHandler dengan hub dan pemeriksa akses antrian.
func NewSSEHandler(
	hub *sse.Hub,
	queues QueueValidator,
) *SSEHandler {
	// isi handler dengan hub dan pemeriksa akses
	return &SSEHandler{
		hub:    hub,
		queues: queues,
	}
}

// ServeSSE: GET /api/v1/queues/:queue_id/event. Membuka aliran event untuk antrian dan menahannya sampai
// koneksi putus.
func (h *SSEHandler) ServeSSE(
	c *gin.Context,
) {

	// ambil ID antrian dari URL dan ID user dari token login
	queueIDStr := c.Param("queue_id")
	requesterID := c.GetInt64("user_id")

	// ubah ID antrian menjadi angka 64-bit; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(
		queueIDStr,
		10,
		64,
	)
	if err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid queue id",
		)
		return
	}

	// pemeriksa akses dipasang: pastikan user boleh melihat antrian ini sebelum membuka stream
	if h.queues != nil {

		// ambil antrian untuk memeriksa hak akses; error berarti ditolak
		if _, _, err := h.queues.ViewQueue(
			c.Request.Context(),
			queueID,
			requesterID,
		); err != nil {
			// antrian tidak ada atau user tidak berhak: 404 agar keberadaannya tidak bocor
			if errors.Is(err, apperror.ErrNotFound) {
				respondError(
					c,
					http.StatusNotFound,
					"NOT_FOUND",
					"queue not found",
				)
				return
			}
			// error lain: 500 tanpa detail
			respondError(
				c,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"internal server error",
			)
			return
		}
	}

	// header khusus SSE: tipe stream, jangan di-cache, dan jaga koneksi tetap terbuka
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	// Cegah nginx/reverse proxy mem-buffer stream.
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// buat klien SSE untuk antrian ini
	client := sse.NewClient(
		h.hub,
		c.Writer,
		queueID,
	)
	// daftarkan klien ke hub; false berarti hub sudah berhenti atau antrean penuh
	ok := h.hub.Subscribe(client)
	if !ok {
		respondError(
			c,
			http.StatusServiceUnavailable,
			"SERVICE_UNAVAILABLE",
			"service is not available",
		)
		return
	}
	// Lepas klien dari room saat koneksi putus, supaya tidak menumpuk sampai buffer-nya penuh.
	defer h.hub.Unsubscribe(client)
	// kirim event ke klien sampai koneksi putus atau server berhenti (fungsi ini memblokir)
	client.Serve(c.Request)
}
