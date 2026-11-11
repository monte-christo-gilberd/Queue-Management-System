package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SubscriptionService adalah logika penambahan masa langganan (dipenuhi service.SubscriptionService).
type SubscriptionService interface {
	// Extend menambah 30 hari ke langganan requesterID dan mengembalikan tanggal berakhir yang baru.
	Extend(ctx context.Context, requesterID int64) (time.Time, error)
}

// SubscriptionHandler menangani rute langganan owner.
type SubscriptionHandler struct {
	// service berisi logika penambahan masa langganan.
	service SubscriptionService
}

// NewSubscriptionHandler membuat SubscriptionHandler dengan service yang diberikan.
func NewSubscriptionHandler(
	service SubscriptionService,
) *SubscriptionHandler {
	return &SubscriptionHandler{
		service: service,
	}
}

// Extend: POST /api/v1/owners/subscription/extend — tambah 30 hari langsung, tanpa pembayaran.
// Calon owner (belum punya baris langganan) otomatis menjadi owner dengan 30 hari pertama.
func (h *SubscriptionHandler) Extend(c *gin.Context) {
	// tambah 30 hari untuk user yang login; error database diterjemahkan jadi 500 generik
	expiresAt, err := h.service.Extend(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// kirim tanggal berakhir yang baru agar frontend bisa menampilkannya
	respondData(c, http.StatusOK, gin.H{"subscription_expires_at": expiresAt})
}
