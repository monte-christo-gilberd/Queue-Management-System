package router

import "github.com/gin-gonic/gin"

// registerRenderRoutes: render QR umum menjadi PNG (khusus backend, lewat X-Service-Token).
func registerRenderRoutes(
	rg *gin.RouterGroup,
	renderH RenderHandler,
) {
	// render data menjadi gambar PNG
	rg.POST("/qr/render", renderH.Render)
}
