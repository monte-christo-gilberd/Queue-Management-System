package middleware

import "github.com/gin-gonic/gin"

// NoStore mencegah cache untuk respons yang berisi token atau data sensitif.
func NoStore() gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// larang klien dan proxy menyimpan respons
		c.Header("Cache-Control", "no-store")
		// header untuk klien HTTP/1.0 yang juga tidak boleh cache
		c.Header("Pragma", "no-cache")
		// lanjutkan ke handler berikutnya
		c.Next()
	}
}
