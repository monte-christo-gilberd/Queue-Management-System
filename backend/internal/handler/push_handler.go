package handler

import (
	"context"
	"net/http"
	"net/url"
	"qms/internal/dto"

	"github.com/gin-gonic/gin"
)

type PushStore interface {
	Upsert(ctx context.Context, userID int64, endpoint, p256dh, auth string) error
	Delete(ctx context.Context, userID int64, endpoint string) error
}

type PushHandler struct {
	store     PushStore
	publicKey string
}

func NewPushHandler(store PushStore, publicKey string) *PushHandler {
	return &PushHandler{store: store, publicKey: publicKey}
}

// PublicKey: GET /api/v1/push/public-key — kunci VAPID publik untuk pushManager.subscribe.
func (h *PushHandler) PublicKey(c *gin.Context) {
	respondData(c, http.StatusOK, gin.H{"public_key": h.publicKey})
}

// Subscribe: POST /api/v1/push/subscribe
func (h *PushHandler) Subscribe(c *gin.Context) {
	var req dto.PushSubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// Push service hanya melayani endpoint HTTPS.
	if u, err := url.Parse(req.Endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid endpoint")
		return
	}
	if err := h.store.Upsert(c.Request.Context(), c.GetInt64("user_id"), req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	respondData(c, http.StatusOK, gin.H{"subscribed": true})
}

// Unsubscribe: POST /api/v1/push/unsubscribe
func (h *PushHandler) Unsubscribe(c *gin.Context) {
	var req dto.PushUnsubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if err := h.store.Delete(c.Request.Context(), c.GetInt64("user_id"), req.Endpoint); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
	respondData(c, http.StatusOK, gin.H{"subscribed": false})
}
