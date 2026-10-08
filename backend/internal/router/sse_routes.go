package router

import (
	"github.com/gin-gonic/gin"
)

func registerSSERoutes(
	rg *gin.RouterGroup,
	sseH SSEHandler,
) {
	if sseH == nil {
		return
	}
	rg.GET("/:queue_id/event", sseH.ServeSSE)
}
