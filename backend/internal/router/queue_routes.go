package router

import "github.com/gin-gonic/gin"

func registerQueueRoutes(
	rg *gin.RouterGroup,
	queueH QueueHandler,
	ticketH TicketHandler,
) {
	rg.POST("", queueH.Create)
	rg.GET("", queueH.List)

	// QR operator: status, mode, dan penggantian kode statis.
	rg.GET("/:queue_id/qr", queueH.GetQR)
	rg.PUT("/:queue_id/qr/type", queueH.SetQRType)
	rg.POST("/:queue_id/qr/replace", queueH.ReplaceQR)

	rg.GET("/:queue_id/settings/skip", queueH.GetSkipSettings)
	rg.PUT("/:queue_id/settings/skip", queueH.UpdateSkipSettings)

	// Join berbasis token: nomor id tidak dipakai lagi dari URL.
	rg.POST("/:queue_id/next", ticketH.Next)
	rg.POST("/:queue_id/skip", ticketH.Skip)
	rg.POST("/:queue_id/recall", ticketH.Recall)
	rg.POST("/:queue_id/cancel", ticketH.Cancel)
	rg.POST("/:queue_id/tickets/:ticket_id/remove", ticketH.Remove)
	rg.GET("/:queue_id/tickets", ticketH.List)
	rg.GET("/:queue_id/status", ticketH.Status)

	rg.GET("/:queue_id", queueH.Get)
	rg.PUT("/:queue_id", queueH.Update)
	rg.POST("/:queue_id/close", queueH.Close)
}
