package router

import (
	"github.com/gin-gonic/gin"
)

// registerSSERoutes mendaftarkan rute koneksi SSE antrian. Bila sseH nil, tidak ada rute yang didaftarkan.
func registerSSERoutes(
	rg *gin.RouterGroup,
	sseH SSEHandler,
) {
	// handler tidak dipasang: lewati pendaftaran rute
	if sseH == nil {
		return
	}
	// buka koneksi SSE untuk antrian
	rg.GET("/:queue_id/event", sseH.ServeSSE)
}
