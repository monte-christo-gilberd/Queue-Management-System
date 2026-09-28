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

type AppRouter struct {
	ticketHandler  TicketHandler
	userHandler    UserHandler
	authMiddleware gin.HandlerFunc
}

func NewAppRouter(
	ticketHandler TicketHandler,
	userHandler UserHandler,
	authMiddleware gin.HandlerFunc,
) *AppRouter {
	return &AppRouter{
		ticketHandler:  ticketHandler,
		userHandler:    userHandler,
		authMiddleware: authMiddleware,
	}
}

func SetupRouter(router *AppRouter) *gin.Engine {
	engine := gin.Default()
	api := engine.Group("/api")

	users := api.Group("/users")
	{
		users.POST("/register", router.userHandler.Register)
		users.POST("/login", router.userHandler.Login)
	}

	queue := api.Group("queues/:queue_id")
	queue.Use(router.authMiddleware)
	{
		queue.POST("/join", router.ticketHandler.Join)
		queue.POST("/next", router.ticketHandler.Next)
	}

	return engine
}
