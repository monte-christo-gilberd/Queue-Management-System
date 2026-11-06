package middleware

import "github.com/gin-gonic/gin"

// abortError menghentikan request dan langsung membalas dengan status serta body JSON error.
// Bentuk body-nya {"error": {"code", "message"}}, sama dengan bentuk error di handler, sehingga klien
// menanganinya dengan cara yang sama. Setelah dipanggil, handler berikutnya dalam rantai tidak dijalankan.
func abortError(
	c *gin.Context,
	status int,
	code, message string,
) {
	// menghentikan rantai middleware dan mengirim status beserta body JSON error
	c.AbortWithStatusJSON(
		status, gin.H{
			"error": gin.H{
				"code":    code,
				"message": message,
			},
		},
	)
}
