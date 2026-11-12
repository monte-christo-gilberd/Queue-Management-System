package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ServiceToken menolak request yang X-Service-Token-nya tidak cocok dengan token layanan (perbandingan dalam
// waktu tetap).
func ServiceToken(
	token string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil token dari header request.
		got := c.GetHeader("X-Service-Token")
		// Token tidak cocok: tolak dengan 401 dan jangan jalankan handler. Perbandingan berwaktu tetap supaya
		// lama respons tidak membocorkan seberapa banyak awal token yang benar.
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			c.Header("Cache-Control", "no-store")
			c.AbortWithStatusJSON(
				http.StatusUnauthorized,
				gin.H{
					"error": gin.H{
						"code":    "UNAUTHORIZED",
						"message": "invalid service token",
					},
				},
			)
			return
		}
		// Token cocok: jalankan handler berikutnya.
		c.Next()
	}
}
