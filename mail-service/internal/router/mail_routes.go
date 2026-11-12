package router

import "github.com/gin-gonic/gin"

// registerMailRoutes mendaftarkan endpoint kirim email. Satu-satunya rute yang dilindungi X-Service-Token.
func registerMailRoutes(
	engine *gin.Engine,
	h MailHandler,
	auth gin.HandlerFunc,
) {
	// Endpoint kirim: token diperiksa dulu oleh middleware sebelum handler.
	engine.POST("/internal/v1/send", auth, h.Send)
}
