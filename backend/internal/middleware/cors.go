package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS hanya memberi izin ke origin yang terdaftar di allowedOrigins.
// Origin lain tidak mendapat header CORS (browser akan menolak). "*" di daftar
// berarti semua origin diizinkan dan hanya boleh dipakai di dev.
func CORS(
	allowedOrigins []string,
) gin.HandlerFunc {

	originMap := make(map[string]bool)
	allowAll := false
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAll = true
			continue
		}
		originMap[origin] = true
	}

	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")

		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || originMap[origin]) {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header(
				"Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS",
			)
			c.Header(
				"Access-Control-Allow-Headers",
				"Origin, Content-Type, Accept, Authorization, X-Requested-With",
			)
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
