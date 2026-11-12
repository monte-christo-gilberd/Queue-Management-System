package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"qms/mail-service/internal/dto"
)

// SystemHandler: handler untuk rute non-fitur, yaitu health check dan 404. Dibuat lewat NewSystemHandler.
type SystemHandler struct{}

// NewSystemHandler membuat SystemHandler.
func NewSystemHandler() *SystemHandler {
	// handler ini tidak punya state
	return &SystemHandler{}
}

// Health menangani GET /health: selalu 200 selama proses hidup, tanpa menyentuh SMTP atau database.
func (h *SystemHandler) Health(
	c *gin.Context,
) {
	// balas status ok tanpa pembungkus "data"
	respondJSON(
		c,
		http.StatusOK,
		dto.HealthResponse{
			Status: "ok",
		},
	)
}

// NotFound menjawab rute yang tidak dikenal dengan 404 dalam format error yang seragam.
func (h *SystemHandler) NotFound(
	c *gin.Context,
) {
	// balas 404 dengan format error standar
	respondError(
		c,
		http.StatusNotFound,
		"NOT_FOUND",
		"route not found",
	)
}
