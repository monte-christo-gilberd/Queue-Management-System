package handler

import "github.com/gin-gonic/gin"

// respondData menulis respons sukses {"data": ...}.
func respondData(
	c *gin.Context,
	status int,
	data any,
) {
	// bungkus data di field "data"
	c.JSON(status, gin.H{"data": data})
}

// respondError menulis respons galat {"error": {"code", "message"}}. code adalah kode tetap yang dibaca klien.
func respondError(
	c *gin.Context,
	status int,
	code,
	message string,
) {
	// simpan kode galat di context agar middleware metrik bisa membacanya setelah handler selesai
	c.Set("error_code", code)
	// bungkus kode dan pesan di field "error"
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}
