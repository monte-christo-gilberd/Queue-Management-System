package router

import "github.com/gin-gonic/gin"

// registerTokenRoutes: pembuatan dan verifikasi token join (khusus backend, lewat X-Service-Token).
func registerTokenRoutes(
	rg *gin.RouterGroup,
	tokenH TokenHandler,
) {
	// buat token join baru
	rg.POST("/join-token", tokenH.IssueToken)
	// periksa tanda tangan dan masa berlaku token
	rg.POST("/join-token/verify", tokenH.VerifyToken)
}
