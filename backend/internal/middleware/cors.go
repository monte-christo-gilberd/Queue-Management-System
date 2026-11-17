package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS hanya memberi izin ke origin yang terdaftar di allowedOrigins.
// Origin lain tidak mendapat header CORS (browser akan menolak). "*" di daftar
// berarti semua origin diizinkan dan hanya boleh dipakai di dev.
//
// Daftar origin diproses sekali saat CORS dipanggil (di router), lalu fungsi yang dikembalikan dipakai untuk setiap request.
// Dipasang di router (internal/router/router.go) sebelum rute lain.
func CORS(
	allowedOrigins []string,
) gin.HandlerFunc {

	// menyiapkan daftar origin yang diizinkan agar pencariannya cepat
	originMap := make(map[string]bool)
	allowAll := false

	// memisahkan "*" (izinkan semua) dari origin tertentu
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			continue
		}
		originMap[origin] = true
	}

	// fungsi yang dijalankan untuk setiap request
	return func(c *gin.Context) {

		// menandai bahwa jawaban bergantung pada header Origin, agar cache tidak mencampur origin
		c.Writer.Header().Add("Vary", "Origin")

		// membaca origin dari request; origin yang tidak terdaftar tidak mendapat header CORS
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || originMap[origin]) {

			// mengizinkan semua origin bila "*", atau hanya origin yang diminta bila terdaftar
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}

			// metode dan header yang boleh dipakai browser saat memanggil API
			c.Header(
				"Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS",
			)
			c.Header(
				"Access-Control-Allow-Headers",
				"Origin, Content-Type, Accept, Authorization, X-Requested-With",
			)
		}

		// preflight (OPTIONS) dijawab langsung tanpa memanggil handler endpoint
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		// meneruskan request ke middleware dan handler berikutnya
		c.Next()
	}
}
