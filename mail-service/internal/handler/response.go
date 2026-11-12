package handler

import "github.com/gin-gonic/gin"

// respondJSON menulis respons JSON dengan status tertentu dan melarang cache.
func respondJSON(
	c *gin.Context,
	status int,
	v any,
) {
	// Larang cache lalu tulis nilai sebagai JSON (Content-Type diatur gin).
	c.Header("Cache-Control", "no-store")
	c.JSON(status, v)
}

// respondData menulis respons sukses dengan isi dibungkus di field "data".
func respondData(
	c *gin.Context,
	status int,
	v any,
) {
	// Bungkus nilai ke field "data" lalu tulis sebagai JSON.
	respondJSON(
		c,
		status,
		gin.H{
			"data": v,
		},
	)
}

// respondError menulis respons error dengan kode dan pesan dalam format yang seragam.
func respondError(
	c *gin.Context,
	status int,
	code string,
	msg string,
) {
	// Bungkus kode dan pesan ke bentuk error standar lalu tulis sebagai JSON.
	respondJSON(
		c,
		status,
		gin.H{
			"error": gin.H{
				"code":    code,
				"message": msg,
			},
		},
	)
}
