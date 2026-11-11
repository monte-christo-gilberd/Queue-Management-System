package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServiceToken menolak request internal yang tidak membawa X-Service-Token yang benar.
// Perbandingan memakai constant time agar tidak bisa ditebak lewat timing.
func ServiceToken(
	expected string,
) gin.HandlerFunc {
	// ubah token yang diharapkan ke byte sekali di awal, supaya tidak diubah ulang di setiap request
	expectedBytes := []byte(expected)
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// ambil token dari header request; header kosong menghasilkan string kosong
		provided := c.GetHeader("X-Service-Token")
		// bandingkan dalam waktu tetap; hasil bukan 1 berarti token salah, tolak dengan 401
		if subtle.ConstantTimeCompare([]byte(provided), expectedBytes) != 1 {
			abortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid service token")
			return
		}
		// token benar: lanjutkan ke handler berikutnya
		c.Next()
	}
}
