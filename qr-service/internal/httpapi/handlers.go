package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"qms/qr-service/internal/joinlink"
	"qms/qr-service/internal/qr"
)

const (
	maxGenericDataLength = 1024
	minCustomTTL         = 30 * time.Second
	maxCustomTTL         = 10 * time.Minute
)

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

// Handlers memegang dependensi endpoint publik dan internal.
type Handlers struct {
	signer     *joinlink.Signer
	dynamicTTL time.Duration
	now        func() time.Time
}

func NewHandlers(
	signer *joinlink.Signer,
	dynamicTTL time.Duration,
) *Handlers {
	return &Handlers{
		signer:     signer,
		dynamicTTL: dynamicTTL,
		now:        time.Now,
	}
}

// Health menjawab liveness probe.
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready menjawab readiness probe. Konfigurasi sudah divalidasi saat start, jadi
// service yang berjalan selalu siap melayani.
func (h *Handlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

type issueTokenRequest struct {
	QueueID    int64  `json:"queue_id"`
	Type       string `json:"type"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type tokenResponse struct {
	Token     string `json:"token"`
	Type      string `json:"type"`
	ExpiresAt *int64 `json:"expires_at"`
}

// IssueToken membuat token join baru. Dipakai backend saat membuat QR statis
// pertama kali atau saat meminta token dinamis.
func (h *Handlers) IssueToken(c *gin.Context) {
	var req issueTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.QueueID <= 0 {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "queue_id must be positive")
		return
	}

	typ := joinlink.Type(req.Type)
	if !typ.Valid() {
		abortWithError(c, http.StatusBadRequest, "QR_TYPE_INVALID", "type must be static or dynamic")
		return
	}

	ttl := h.dynamicTTL
	if typ == joinlink.TypeDynamic && req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
		if ttl < minCustomTTL || ttl > maxCustomTTL {
			abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "ttl_seconds out of range")
			return
		}
	}

	token, claims, err := h.signer.Issue(req.QueueID, typ, ttl)
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "could not issue token")
		return
	}

	resp := tokenResponse{Token: token, Type: string(claims.Type)}
	if claims.ExpiresAt > 0 {
		exp := claims.ExpiresAt
		resp.ExpiresAt = &exp
	}
	c.JSON(http.StatusOK, resp)
}

type verifyTokenRequest struct {
	Token string `json:"token"`
}

type verifyTokenResponse struct {
	Valid     bool   `json:"valid"`
	QueueID   int64  `json:"queue_id"`
	Type      string `json:"type"`
	ExpiresAt *int64 `json:"expires_at"`
}

// VerifyToken memeriksa tanda tangan dan masa berlaku token. Backend tetap wajib
// mencocokkan token statis dengan yang tersimpan di queue_settings.
func (h *Handlers) VerifyToken(c *gin.Context) {
	var req verifyTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	claims, err := h.signer.Verify(req.Token)
	if err != nil {
		abortForTokenError(c, err)
		return
	}

	resp := verifyTokenResponse{
		Valid:   true,
		QueueID: claims.QueueID,
		Type:    string(claims.Type),
	}
	if claims.ExpiresAt > 0 {
		exp := claims.ExpiresAt
		resp.ExpiresAt = &exp
	}
	c.JSON(http.StatusOK, resp)
}

type renderRequest struct {
	Data  string `json:"data"`
	Size  int    `json:"size"`
	Level string `json:"level"`
	FG    string `json:"fg"`
	BG    string `json:"bg"`
}

// RenderGeneric merender QR apa pun untuk kebutuhan internal lain (mis. QR
// pembayaran). Tidak menerima data rahasia dari klien publik.
func (h *Handlers) RenderGeneric(c *gin.Context) {
	var req renderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if req.Data == "" || len(req.Data) > maxGenericDataLength {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", "data is required (max 1024 characters)")
		return
	}

	png, err := qr.Render(qr.Options{
		Data:            req.Data,
		Size:            req.Size,
		RecoveryLevel:   req.Level,
		ForegroundColor: req.FG,
		BackgroundColor: req.BG,
	})
	if err != nil {
		abortWithError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	c.Header("Cache-Control", "no-store")
	writePNG(c, png)
}

func writePNG(c *gin.Context, png []byte) {
	c.Header("Content-Type", "image/png")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/png", png)
}

func abortForTokenError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, joinlink.ErrExpired):
		abortWithError(c, http.StatusGone, "QR_EXPIRED", "token expired")
	case errors.Is(err, joinlink.ErrInvalidSignature):
		abortWithError(c, http.StatusUnauthorized, "QR_INVALID", "invalid token")
	case errors.Is(err, joinlink.ErrMalformed),
		errors.Is(err, joinlink.ErrUnsupportedFormat),
		errors.Is(err, joinlink.ErrInvalidPayload):
		abortWithError(c, http.StatusBadRequest, "QR_MALFORMED", "malformed token")
	default:
		abortWithError(c, http.StatusBadRequest, "QR_MALFORMED", "invalid token")
	}
}

func abortWithError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, errorResponse{Error: errorBody{Code: code, Message: message}})
}
