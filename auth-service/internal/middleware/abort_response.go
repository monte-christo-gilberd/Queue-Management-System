package middleware

import "github.com/gin-gonic/gin"

// abortError menghentikan request dengan body JSON error {"error": {"code", "message"}} yang seragam.
func abortError(
	c *gin.Context,
	status int,
	code,
	message string,
) {
	// tulis body error, lalu hentikan rantai handler
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
	// Abort menjamin handler berikutnya tidak berjalan
	c.Abort()
}
