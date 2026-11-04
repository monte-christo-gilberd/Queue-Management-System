package handler

import "github.com/gin-gonic/gin"

type QueueHandlerInterface interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Close(c *gin.Context)
	Info(c *gin.Context)
	GetQR(c *gin.Context)
	SetQRType(c *gin.Context)
	ReplaceQR(c *gin.Context)
}

type TicketHandlerInterface interface {
	Join(c *gin.Context)
	Preview(c *gin.Context)
	Next(c *gin.Context)
	Skip(c *gin.Context)
	Cancel(c *gin.Context)
	Remove(c *gin.Context)
	Insert(c *gin.Context)
	List(c *gin.Context)
	Status(c *gin.Context)
	MyTickets(c *gin.Context)
}

type UserHandlerInterface interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Me(c *gin.Context)
	UpdateMe(c *gin.Context)
	UploadAvatar(c *gin.Context)
	GetAvatar(c *gin.Context)
	DeleteAvatar(c *gin.Context)
}

type SSEHandlerInterface interface {
	ServeSSE(c *gin.Context)
}
