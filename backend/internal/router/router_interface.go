package router

import "github.com/gin-gonic/gin"

type TicketHandler interface {
	Join(c *gin.Context)
	Next(c *gin.Context)
}

type UserHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
}

type QueueHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	UpdateAuthz(c *gin.Context)
	Close(c *gin.Context)
}

type SSEHandler interface {
	ServeSSE(c *gin.Context)
}

type AppRouter struct {
	ticketHandler  TicketHandler
	userHandler    UserHandler
	queueHandler   QueueHandler
	sseHandler     SSEHandler
	authMiddleware gin.HandlerFunc
	allowedOrigins []string
}
