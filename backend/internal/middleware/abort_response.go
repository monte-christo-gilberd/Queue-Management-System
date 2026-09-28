package middleware

import "github.com/gin-gonic/gin"

func abortError(
	c *gin.Context,
	status int,
	code, message string,
) {
	c.AbortWithStatusJSON(
		status, gin.H{
			"error": gin.H{
				"code":    code,
				"message": message,
			},
		},
	)
}
