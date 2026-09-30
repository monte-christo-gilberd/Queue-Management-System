package router

import "github.com/gin-gonic/gin"


func NewAppRouter(
	ticketHandler TicketHandler,
	userHandler UserHandler,
	queueHandler QueueHandler,
	authMiddleware gin.HandlerFunc,
) *AppRouter {
	return &AppRouter{
		ticketHandler:  ticketHandler,
		userHandler:    userHandler,
		queueHandler:   queueHandler,
		authMiddleware: authMiddleware,
	}
}

func SetupRouter(router *AppRouter) *gin.Engine {
	engine := gin.Default()
	api := engine.Group("/api")

	users := api.Group("/users")
	
	queues := api.Group("/queues")
	queues.Use(router.authMiddleware)
	
	queue := queues.Group("/:queue_id")
	

	{
		users.POST("/register", router.userHandler.Register)
		users.POST("/login", router.userHandler.Login)
	}

	{
		queues.POST("", router.queueHandler.Create)
		queues.GET("", router.queueHandler.List)
	}

	{
		queue.POST("/join", router.ticketHandler.Join)
		queue.POST("/next", router.ticketHandler.Next)

		queue.GET("", router.queueHandler.Get)
		queue.PUT("", router.queueHandler.Update)
		queue.PUT("/authz", router.queueHandler.UpdateAuthz)
		queue.POST("/close", router.queueHandler.Close)
	}

	return engine
}
