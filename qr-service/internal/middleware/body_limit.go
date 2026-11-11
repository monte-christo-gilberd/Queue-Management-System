package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit menolak body yang lebih besar dari maxBytes. Endpoint internal hanya
// menerima JSON kecil, jadi batas ketat mencegah penyalahgunaan memori.
func BodyLimit(
	maxBytes int64,
) gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// panjang body yang diumumkan klien melebihi batas: tolak sebelum membaca apa pun
		if c.Request.ContentLength > maxBytes {
			abortWithError(c, http.StatusRequestEntityTooLarge, "BODY_TOO_LARGE", "request body too large")
			return
		}
		// bungkus body agar pembacaan berhenti di batas, walau klien tidak mengumumkan panjangnya
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		// lanjutkan ke handler berikutnya
		c.Next()
	}
}
