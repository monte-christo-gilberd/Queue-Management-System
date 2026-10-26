package handler

import "github.com/gin-gonic/gin"

type QueueHandlerInterface interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Close(c *gin.Context)
}

type TicketHandlerInterface interface {
	Join(c *gin.Context)
	Next(c *gin.Context)
	List(c *gin.Context)
	Status(c *gin.Context)
}

type UserHandlerInterface interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
}

type SSEHandlerInterface interface {
	ServeSSE(c *gin.Context)
}
