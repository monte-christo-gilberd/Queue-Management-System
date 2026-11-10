package handler

import (
	"github.com/gin-gonic/gin"
)

// respondData menulis respons sukses dengan status HTTP status dan isi JSON {"data": ...}.
func respondData(
	c *gin.Context,
	status int,
	data interface{},
) {
	// bungkus data dalam field "data" lalu kirim sebagai JSON
	c.JSON(
		status,
		gin.H{
			"data": data,
		},
	)
}

// respondError menulis respons error dengan status HTTP status dan isi JSON {"error": {"code", "message"}}.
// code adalah kode tetap yang dibaca klien, message adalah penjelasan singkat.
func respondError(
	c *gin.Context,
	status int,
	code, message string,
) {
	// bungkus kode dan pesan dalam field "error" lalu kirim sebagai JSON
	c.JSON(
		status,
		gin.H{
			"error": gin.H{
				"code":    code,
				"message": message,
			},
		},
	)
}
