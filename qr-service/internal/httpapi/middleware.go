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
	// Ubah token yang diharapkan ke byte sekali di awal, supaya tidak diubah ulang di setiap request.
	expectedBytes := []byte(expected)
	return func(c *gin.Context) {
		// Ambil token dari header request. Header kosong menghasilkan string kosong.
		provided := c.GetHeader("X-Service-Token")
		// Bandingkan dalam waktu tetap. Hasil bukan 1 berarti token salah: tolak dengan 401.
		if subtle.ConstantTimeCompare([]byte(provided), expectedBytes) != 1 {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
			return
		}
		// Token benar: lanjutkan ke handler berikutnya.
		c.Next()
	}
}

// BodyLimit menolak body yang lebih besar dari maxBytes. Endpoint internal hanya
// menerima JSON kecil, jadi batas ketat mencegah penyalahgunaan memori.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Panjang body yang diumumkan klien melebihi batas: tolak sebelum membaca apa pun.
		if c.Request.ContentLength > maxBytes {
			abortWithError(c, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "request body too large")
			return
		}
		// Bungkus body agar pembacaan berhenti di batas, walau klien tidak mengumumkan panjangnya.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		// Lanjutkan ke handler berikutnya.
		c.Next()
	}
}

// RequestID memberi setiap request id untuk korelasi log. Nilai dari header
// X-Request-ID dipakai bila ada (mis. diteruskan backend), selain itu dibuat baru.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil request ID dari header yang diteruskan backend.
		id := c.GetHeader("X-Request-ID")
		// Header kosong atau terlalu panjang (di atas 128 karakter): buat ID baru.
		if id == "" || len(id) > 128 {
			id = newRequestID()
		}
		// Simpan ID di konteks agar Logger bisa mencatatnya.
		c.Set("request_id", id)
		// Kirim ID kembali ke klien untuk korelasi.
		c.Header("X-Request-ID", id)
		// Lanjutkan ke handler berikutnya.
		c.Next()
	}
}

// Logger mencatat satu baris JSON per request. Query string sengaja tidak dicatat
// karena berisi token join; hanya path yang dicatat.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Catat waktu mulai, lalu jalankan handler dan middleware berikutnya.
		start := time.Now()
		c.Next()

		// Kumpulkan atribut log setelah request selesai, termasuk status dan durasi.
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("request_id"),
			"ip", c.ClientIP(),
		}
		// Pilih level log sesuai status: 5xx galat, 4xx peringatan, selain itu info.
		switch {
		// Status 5xx: kesalahan server, catat sebagai error.
		case c.Writer.Status() >= 500:
			logger.Error("request", attrs...)
		// Status 4xx: kesalahan dari klien, catat sebagai warning.
		case c.Writer.Status() >= 400:
			logger.Warn("request", attrs...)
		// Status lain: catat sebagai info.
		default:
			logger.Info("request", attrs...)
		}
	}
}

// NoStore mencegah cache untuk respons yang berisi token atau data sensitif.
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Larang klien dan proxy menyimpan respons.
		c.Header("Cache-Control", "no-store")
		// Header untuk klien HTTP/1.0 yang juga tidak boleh cache.
		c.Header("Pragma", "no-cache")
		// Lanjutkan ke handler berikutnya.
		c.Next()
	}
}

// newRequestID membuat ID acak 12 byte dalam bentuk hex (24 karakter). Bila sumber acak gagal, kembalikan "unknown".
func newRequestID() string {
	// Siapkan buffer 12 byte acak.
	buf := make([]byte, 12)
	// Isi dengan byte acak dari crypto/rand; bila gagal, pakai nilai penanda "unknown".
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	// Ubah ke hex agar aman ditulis di header dan log.
	return hex.EncodeToString(buf)
}
