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
	rg.GET("/:queue_id/tickets", ticketH.List)
	rg.GET("/:queue_id/status", ticketH.Status)

	rg.GET("/:queue_id", queueH.Get)
	rg.GET("/:queue_id/info", queueH.Info)
	rg.PUT("/:queue_id", queueH.Update)
	rg.POST("/:queue_id/close", queueH.Close)
}
