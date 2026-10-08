package router

import (
	"github.com/gin-gonic/gin"
)

func NewAppRouter(
	ticketHandler TicketHandler,
	userHandler UserHandler,
	queueHandler QueueHandler,
	authMiddleware gin.HandlerFunc,
	sseHandlers SSEHandler,
) *AppRouter {
	app := &AppRouter{
		ticketHandler:  ticketHandler,
		userHandler:    userHandler,
		queueHandler:   queueHandler,
		authMiddleware: authMiddleware,
		sseHandler: sseHandlers,
	}
	return app
}

func SetupRouter(
	router *AppRouter,
) *gin.Engine {
	engine := gin.Default()
	api := engine.Group("/api")
	v1 := api.Group("/v1")

	users := v1.Group("/users")
	registerUserRoutes(
		users,
		router.userHandler,
	)

	queues := v1.Group("/queues")
	queues.Use(router.authMiddleware)
	registerQueueRoutes(
		queues,
		router.queueHandler,
		router.ticketHandler,
	)
	registerSSERoutes(
		queues,
		router.sseHandler,
	)

	return engine
}
