package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler menangani probe liveness dan readiness.
type HealthHandler struct{}

// NewHealthHandler membuat HealthHandler.
func NewHealthHandler() *HealthHandler {
	// handler tanpa dependency
	return &HealthHandler{}
}

// Health menjawab liveness probe.
func (h *HealthHandler) Health(c *gin.Context) {
	// balas 200 dengan status ok agar orkestrasi tahu proses masih hidup
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready menjawab readiness probe. Konfigurasi sudah divalidasi saat start, jadi
// service yang berjalan selalu siap melayani.
func (h *HealthHandler) Ready(c *gin.Context) {
	// balas 200 dengan status ready; service sudah siap karena konfigurasi divalidasi saat start
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
