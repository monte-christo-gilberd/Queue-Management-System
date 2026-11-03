package router

import "github.com/gin-gonic/gin"

type TicketHandler interface {
	Join(c *gin.Context)
	Preview(c *gin.Context)
	Next(c *gin.Context)
	Skip(c *gin.Context)
	Cancel(c *gin.Context)
	Recall(c *gin.Context)
	List(c *gin.Context)
	Status(c *gin.Context)
	MyTickets(c *gin.Context)
}

type UserHandler interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	Me(c *gin.Context)
}

type QueueHandler interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Close(c *gin.Context)
	GetQR(c *gin.Context)
	SetQRType(c *gin.Context)
	ReplaceQR(c *gin.Context)
	GetSkipSettings(c *gin.Context)
	UpdateSkipSettings(c *gin.Context)
}

type PushHandler interface {
	PublicKey(c *gin.Context)
	Subscribe(c *gin.Context)
	Unsubscribe(c *gin.Context)
}

type SSEHandler interface {
	ServeSSE(c *gin.Context)
}

type NoteHandler interface {
	Get(c *gin.Context)
	Save(c *gin.Context)
}

type AppRouter struct {
	noteHandler    NoteHandler
	pushHandler    PushHandler
	ticketHandler  TicketHandler
	userHandler    UserHandler
	queueHandler   QueueHandler
	sseHandler     SSEHandler
	authMiddleware gin.HandlerFunc
	allowedOrigins []string
}
