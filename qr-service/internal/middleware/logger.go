package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger mencatat satu baris JSON per request: request_id, method, route, status, latency_ms, bytes, client_ip.
// route adalah templat rute Gin, bukan path mentah, dan query string tidak pernah dicatat karena bisa berisi token join.
func Logger(
	logger *slog.Logger,
) gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// catat waktu mulai, lalu jalankan handler dan middleware berikutnya
		start := time.Now()
		c.Next()

		// templat rute yang cocok; kosong berarti tidak ada rute (404), jadi pakai penanda tetap, bukan path mentah
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		// ukuran body respons; Gin memberi -1 bila belum ada yang ditulis
		bytes := c.Writer.Size()
		if bytes < 0 {
			bytes = 0
		}
		status := c.Writer.Status()

		// kumpulkan atribut log setelah request selesai, termasuk status dan durasi
		attrs := []any{
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"route", route,
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"bytes", bytes,
			"client_ip", c.ClientIP(),
		}
		// pilih level log sesuai status: 5xx galat, 4xx peringatan, selain itu info
		switch {
		// status 5xx: kesalahan server, catat sebagai error
		case status >= 500:
			logger.Error("request", attrs...)
		// status 4xx: kesalahan dari klien, catat sebagai warning
		case status >= 400:
			logger.Warn("request", attrs...)
		// probe liveness dipanggil terus-menerus oleh Docker; turunkan ke debug agar log tidak penuh
		case route == "/health" || route == "/ready":
			logger.Debug("request", attrs...)
		// status lain: catat sebagai info
		default:
			logger.Info("request", attrs...)
		}
	}
}
