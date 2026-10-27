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

	// Tiket saya: butuh login, tidak ada di bawah /queues karena tidak terkait satu antrian.
	me := v1.Group("/users/me")
	me.Use(router.authMiddleware)
	if router.userHandler != nil {
		me.GET("", router.userHandler.Me)
	}
	if router.ticketHandler != nil {
		me.GET("/tickets", router.ticketHandler.MyTickets)
	}

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

	// Join berbasis token: customer yang sudah login membuka /join?t=<token>.
	// Preview dan join diberi auth middleware; token sendiri ditahan service.
	join := v1.Group("/join")
	join.Use(router.authMiddleware)
	if router.ticketHandler != nil {
		join.GET("/preview", router.ticketHandler.Preview)
		join.POST("", router.ticketHandler.Join)
	}

	return engine
}
