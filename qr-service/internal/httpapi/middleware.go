package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ServiceToken menolak request internal yang tidak membawa X-Service-Token yang benar.
// Perbandingan memakai constant time agar tidak bisa ditebak lewat timing.
func ServiceToken(expected string) gin.HandlerFunc {
	expectedBytes := []byte(expected)
	return func(c *gin.Context) {
		provided := c.GetHeader("X-Service-Token")
		if subtle.ConstantTimeCompare([]byte(provided), expectedBytes) != 1 {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
			return
		}
		c.Next()
	}
}

// BodyLimit menolak body yang lebih besar dari maxBytes. Endpoint internal hanya
// menerima JSON kecil, jadi batas ketat mencegah penyalahgunaan memori.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			abortWithError(c, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "request body too large")
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// RequestID memberi setiap request id untuk korelasi log. Nilai dari header
// X-Request-ID dipakai bila ada (mis. diteruskan backend), selain itu dibuat baru.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = newRequestID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

// Logger mencatat satu baris JSON per request. Query string sengaja tidak dicatat
// karena berisi token join; hanya path yang dicatat.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("request_id"),
			"ip", c.ClientIP(),
		}
		switch {
		case c.Writer.Status() >= 500:
			logger.Error("request", attrs...)
		case c.Writer.Status() >= 400:
			logger.Warn("request", attrs...)
		default:
			logger.Info("request", attrs...)
		}
	}
}

// NoStore mencegah cache untuk respons yang berisi token atau data sensitif.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}

func newRequestID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}
