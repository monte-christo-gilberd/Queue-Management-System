package handler

import "github.com/gin-gonic/gin"

// HealthHandlerInterface adalah kontrak handler probe container yang dipakai oleh router.
type HealthHandlerInterface interface {
	// Health menjawab liveness probe.
	Health(c *gin.Context)
	// Ready menjawab readiness probe.
	Ready(c *gin.Context)
}

// TokenHandlerInterface adalah kontrak handler token join yang dipakai oleh router.
type TokenHandlerInterface interface {
	// IssueToken membuat token join baru.
	IssueToken(c *gin.Context)
	// VerifyToken memeriksa tanda tangan dan masa berlaku token join.
	VerifyToken(c *gin.Context)
}

// RenderHandlerInterface adalah kontrak handler render QR yang dipakai oleh router.
type RenderHandlerInterface interface {
	// Render merender QR umum menjadi PNG.
	Render(c *gin.Context)
}
