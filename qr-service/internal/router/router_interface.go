package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// HealthHandler adalah handler untuk probe liveness dan readiness.
type HealthHandler interface {
	// Health menjawab liveness probe.
	Health(c *gin.Context)
	// Ready menjawab readiness probe.
	Ready(c *gin.Context)
}

// TokenHandler adalah handler untuk pembuatan dan verifikasi token join.
type TokenHandler interface {
	// IssueToken membuat token join baru.
	IssueToken(c *gin.Context)
	// VerifyToken memeriksa tanda tangan dan masa berlaku token join.
	VerifyToken(c *gin.Context)
}

// RenderHandler adalah handler untuk render QR umum menjadi PNG.
type RenderHandler interface {
	// Render merender QR umum menjadi PNG.
	Render(c *gin.Context)
}

// AppRouter menyimpan semua handler dan konfigurasi yang akan dipasang ke rute. Dibuat lewat NewAppRouter.
// Handler yang berawalan opsional bernilai nil bila tidak dipasang, dan rutenya juga tidak didaftarkan.
type AppRouter struct {
	// renderHandler menangani render QR umum (opsional).
	renderHandler RenderHandler
	// healthHandler menangani probe liveness dan readiness.
	healthHandler HealthHandler
	// tokenHandler menangani pembuatan dan verifikasi token join.
	tokenHandler TokenHandler
	// serviceToken adalah nilai X-Service-Token yang wajib dibawa request internal.
	serviceToken string
	// logger mencatat request. Nil berarti slog default.
	logger *slog.Logger
	// metricsMiddleware mencatat metrik HTTP (opsional; nil berarti tanpa metrik).
	metricsMiddleware gin.HandlerFunc
}
