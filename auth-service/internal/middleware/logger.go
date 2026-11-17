package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger mencatat satu baris JSON per request setelah request selesai. Yang dicatat: request_id, method, route,
// status, latency_ms, bytes, client_ip, dan user_id (angka saja, bila request sudah login).
//
// route adalah TEMPLAT rute (mis. /api/v1/users/login), bukan path mentah, dan query string tidak pernah dicatat.
// Dengan begitu token (reset password, verifikasi email) yang terbawa di path atau query tidak masuk log, dan jumlah nilai route tetap kecil.
func Logger(
	logger *slog.Logger,
) gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// catat waktu mulai, lalu jalankan handler dan middleware berikutnya
		start := time.Now()
		c.Next()

		// context request: membawa span aktif, sehingga log akses memuat trace_id dan span_id
		ctx := c.Request.Context()
		// templat rute yang cocok; kosong berarti tidak ada rute (404), jadi pakai penanda tetap, bukan path mentah
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		// ukuran body respons; Gin memberi -1 bila belum ada yang ditulis
		bytes := max(c.Writer.Size(), 0)
		// status akhir respons (setelah handler dan Recovery selesai)
		status := c.Writer.Status()

		// kumpulkan atribut log setelah request selesai
		attrs := []any{
			"request_id", c.GetString("request_id"),
			"method", c.Request.Method,
			"route", route,
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"bytes", bytes,
			"client_ip", c.ClientIP(),
		}
		// user_id hanya ada bila middleware login sudah menyimpannya; dicatat sebagai angka, tanpa data lain
		if uid, ok := userIDFrom(c); ok {
			attrs = append(attrs, "user_id", uid)
		}

		// pilih level log sesuai status: 5xx galat, 4xx peringatan, selain itu info
		switch {
		// status 5xx: kesalahan server, catat sebagai error
		case status >= 500:
			logger.ErrorContext(ctx, "request", attrs...)
		// status 4xx: kesalahan dari klien, catat sebagai warning
		case status >= 400:
			logger.WarnContext(ctx, "request", attrs...)
		// probe liveness /health dipanggil terus-menerus oleh Docker; turunkan ke debug agar log tidak penuh
		// (auth-service tidak punya rute /ready; pencocokan itu sisa salinan dari backend)
		case route == "/health" || route == "/ready":
			logger.DebugContext(ctx, "request", attrs...)
		// status lain: catat sebagai info
		default:
			logger.InfoContext(ctx, "request", attrs...)
		}
	}
}

// userIDFrom membaca user_id yang disimpan middleware login di context Gin. Mengembalikan false bila tidak ada
// atau bukan angka positif.
func userIDFrom(
	c *gin.Context,
) (int64, bool) {
	// ambil nilai mentah; belum login berarti kuncinya tidak ada
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	// middleware login menyimpan int64; int ikut diterima agar tahan terhadap perubahan tipe
	switch id := v.(type) {
	case int64:
		return id, id > 0
	case int:
		return int64(id), id > 0
	}
	// tipe lain tidak dicatat
	return 0, false
}
