package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORS(
	allowedOrigins []string,
) gin.HandlerFunc {

	originMap := make(map[string]bool)
	for _, origin := range allowedOrigins {
		originMap[origin] = true
	}

	return func(c *gin.Context) {
		// origin := c.GetHeader("Origin")

		// if originMap[origin] {
		c.Header("Access-Control-Allow-Origin", "*")
		// }

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}
