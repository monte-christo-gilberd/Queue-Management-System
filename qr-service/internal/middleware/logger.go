package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger mencatat satu baris JSON per request. Query string sengaja tidak dicatat
// karena berisi token join; hanya path yang dicatat.
func Logger(
	logger *slog.Logger,
) gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// catat waktu mulai, lalu jalankan handler dan middleware berikutnya
		start := time.Now()
		c.Next()

		// kumpulkan atribut log setelah request selesai, termasuk status dan durasi
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("request_id"),
			"ip", c.ClientIP(),
		}
		// pilih level log sesuai status: 5xx galat, 4xx peringatan, selain itu info
		switch {
		// status 5xx: kesalahan server, catat sebagai error
		case c.Writer.Status() >= 500:
			logger.Error("request", attrs...)
		// status 4xx: kesalahan dari klien, catat sebagai warning
		case c.Writer.Status() >= 400:
			logger.Warn("request", attrs...)
		// status lain: catat sebagai info
		default:
			logger.Info("request", attrs...)
		}
	}
}
