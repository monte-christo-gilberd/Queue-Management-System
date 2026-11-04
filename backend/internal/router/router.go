package router

import (
	"net/http"

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

// WithCounters memasang endpoint CRUD counter (opsional; tanpa ini rute /counters tidak ada).
func (a *AppRouter) WithCounters(h CounterHandler) *AppRouter {
	a.counterHandler = h
	return a
}

// WithNotes memasang endpoint catatan per nomor tiket (opsional; tanpa ini rute /note tidak ada).
func (a *AppRouter) WithNotes(h NoteHandler) *AppRouter {
	a.noteHandler = h
	return a
}

// WithHistory memasang endpoint riwayat antrian owner (opsional; tanpa ini rute /history tidak ada).
func (a *AppRouter) WithHistory(h HistoryHandler) *AppRouter {
	a.historyHandler = h
	return a
}

// WithPush memasang endpoint Web Push (opsional; tanpa ini rute /push tidak ada).
func (a *AppRouter) WithPush(h PushHandler) *AppRouter {
	a.pushHandler = h
	return a
}

func SetupRouter(
	router *AppRouter,
) *gin.Engine {
	engine := gin.Default()
	engine.Use(middleware.CORS(router.allowedOrigins))

	// Liveness untuk healthcheck kontainer: tidak menyentuh database atau logika antrian.
	engine.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

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
		me.PUT("", router.userHandler.UpdateMe)
	}
	if router.ticketHandler != nil {
		me.GET("/tickets", router.ticketHandler.MyTickets)
	}
	if router.historyHandler != nil {
		me.GET("/history", router.historyHandler.ListMine)
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
	if router.noteHandler != nil {
		registerNoteRoutes(queues, router.noteHandler)
	}
	if router.counterHandler != nil {
		registerCounterRoutes(queues, router.counterHandler)
	}

	if router.historyHandler != nil {
		history := v1.Group("/history")
		history.Use(router.authMiddleware)
		registerHistoryRoutes(history, router.historyHandler)
	}

	if router.pushHandler != nil {
		push := v1.Group("/push")
		push.Use(router.authMiddleware)
		push.GET("/public-key", router.pushHandler.PublicKey)
		push.POST("/subscribe", router.pushHandler.Subscribe)
		push.POST("/unsubscribe", router.pushHandler.Unsubscribe)
	}

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
