package handler

import (
	"net/http"

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
	w http.ResponseWriter,
	_ *http.Request,
) {
	// balas status ok tanpa pembungkus "data"
	respondJSON(w, http.StatusOK, dto.HealthResponse{Status: "ok"})
}

// NotFound menjawab rute yang tidak dikenal dengan 404 dalam format error yang seragam.
func (h *SystemHandler) NotFound(
	w http.ResponseWriter,
	_ *http.Request,
) {
	// balas 404 dengan format error standar
	respondError(w, http.StatusNotFound, "NOT_FOUND", "route not found")
}
