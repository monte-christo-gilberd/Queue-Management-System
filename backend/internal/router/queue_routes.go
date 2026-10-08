package router

import "github.com/gin-gonic/gin"

func registerQueueRoutes(
	rg *gin.RouterGroup,
	queueH QueueHandler,
	ticketH TicketHandler,
) {
	rg.POST("", queueH.Create)
	rg.GET("", queueH.List)

	rg.POST("/:queue_id/join", ticketH.Join)
	rg.POST("/:queue_id/next", ticketH.Next)

	rg.GET("/:queue_id", queueH.Get)
	rg.PUT("/:queue_id", queueH.Update)
	rg.PUT("/:queue_id/authz", queueH.UpdateAuthz)
	rg.POST("/:queue_id/close", queueH.Close)
}
