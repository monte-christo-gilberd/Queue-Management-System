package handler

import (
	"context"
	"net/http"
	"net/url"
	"qms/internal/dto"

	"github.com/gin-gonic/gin"
)

// PushStore adalah penyimpanan langganan Web Push milik user (dipenuhi oleh repository).
type PushStore interface {
	// Upsert menyimpan langganan; bila endpoint sudah ada, datanya diperbarui.
	Upsert(ctx context.Context, userID int64, endpoint, p256dh, auth string) error
	// Delete menghapus langganan milik user berdasarkan endpoint-nya.
	Delete(ctx context.Context, userID int64, endpoint string) error
}

// PushHandler menangani rute Web Push: kunci publik, berlangganan, dan berhenti berlangganan.
type PushHandler struct {
	// store menyimpan langganan push.
	store PushStore
	// publicKey adalah kunci publik VAPID yang dikirim ke browser.
	publicKey string
}

// NewPushHandler membuat PushHandler dengan penyimpanan store dan kunci publik VAPID publicKey.
func NewPushHandler(store PushStore, publicKey string) *PushHandler {
	// isi handler dengan penyimpanan dan kunci publik
	return &PushHandler{store: store, publicKey: publicKey}
}

// PublicKey: GET /api/v1/push/public-key — kunci VAPID publik untuk pushManager.subscribe.
func (h *PushHandler) PublicKey(c *gin.Context) {
	// kirim kunci publik VAPID ke browser
	respondData(c, http.StatusOK, gin.H{"public_key": h.publicKey})
}

// Subscribe: POST /api/v1/push/subscribe
func (h *PushHandler) Subscribe(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req dto.PushSubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// Push service hanya melayani endpoint HTTPS.
	// endpoint harus bisa di-parse, memakai skema https, dan punya host; jika tidak, tolak
	if u, err := url.Parse(req.Endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid endpoint")
		return
	}
	// simpan langganan milik user yang sedang login; gagal berarti error server
	if err := h.store.Upsert(c.Request.Context(), c.GetInt64("user_id"), req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// berhasil berlangganan
	respondData(c, http.StatusOK, gin.H{"subscribed": true})
}

// Unsubscribe: POST /api/v1/push/unsubscribe
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	// baca body JSON; gagal berarti format request salah
	var req dto.PushUnsubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// hapus langganan milik user yang sedang login; gagal berarti error server
	if err := h.store.Delete(c.Request.Context(), c.GetInt64("user_id"), req.Endpoint); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	// berhasil berhenti berlangganan
	respondData(c, http.StatusOK, gin.H{"subscribed": false})
}
