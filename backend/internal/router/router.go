package router

import (
	"qms/internal/middleware"

	"github.com/gin-gonic/gin"
)

func NewAppRouter(
	ticketHandler TicketHandler,
	userHandler UserHandler,
	queueHandler QueueHandler,
	authMiddleware gin.HandlerFunc,
	sseHandlers SSEHandler,
	allowedOrigins []string,
) *AppRouter {
	app := &AppRouter{
		ticketHandler:  ticketHandler,
		userHandler:    userHandler,
		queueHandler:   queueHandler,
		authMiddleware: authMiddleware,
		sseHandler:     sseHandlers,
		allowedOrigins: allowedOrigins,
	}
	return app
}

func SetupRouter(
	router *AppRouter,
) *gin.Engine {
	engine := gin.Default()
	engine.Use(middleware.CORS(router.allowedOrigins))

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
