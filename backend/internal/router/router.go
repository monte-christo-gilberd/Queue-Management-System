package router

import (
	"log/slog"
	"net/http"

	"qms/internal/middleware"

	"github.com/gin-gonic/gin"
)

// NewAppRouter membuat AppRouter dengan handler wajib: tiket, user, antrian, SSE, dan middleware login.
// Fitur opsional dipasang setelahnya lewat metode With...
func NewAppRouter(
	ticketHandler TicketHandler,
	userHandler UserHandler,
	queueHandler QueueHandler,
	authMiddleware gin.HandlerFunc,
	sseHandlers SSEHandler,
	allowedOrigins []string,
) *AppRouter {
	// isi AppRouter dengan handler dan middleware wajib
	app := &AppRouter{
		ticketHandler:  ticketHandler,
		userHandler:    userHandler,
		queueHandler:   queueHandler,
		authMiddleware: authMiddleware,
		sseHandler:     sseHandlers,
		allowedOrigins: allowedOrigins,
	}
	// kembalikan router yang siap dipasang fitur opsional
	return app
}

// WithLogger memasang logger untuk log akses dan Recovery (opsional; tanpa ini dipakai slog.Default()).
func (a *AppRouter) WithLogger(l *slog.Logger) *AppRouter {
	// simpan logger, lalu kembalikan router yang sama agar bisa dirangkai
	a.logger = l
	return a
}

// WithCounters memasang endpoint CRUD counter (opsional; tanpa ini rute /counters tidak ada).
func (a *AppRouter) WithCounters(h CounterHandler) *AppRouter {
	// simpan handler counter, lalu kembalikan router yang sama agar bisa dirangkai
	a.counterHandler = h
	return a
}

// WithNotes memasang endpoint catatan per nomor tiket (opsional; tanpa ini rute /note tidak ada).
func (a *AppRouter) WithNotes(h NoteHandler) *AppRouter {
	// simpan handler catatan, lalu kembalikan router yang sama
	a.noteHandler = h
	return a
}

// WithHistory memasang endpoint riwayat antrian owner (opsional; tanpa ini rute /history tidak ada).
func (a *AppRouter) WithHistory(h HistoryHandler) *AppRouter {
	// simpan handler riwayat, lalu kembalikan router yang sama
	a.historyHandler = h
	return a
}

// WithDisplay memasang endpoint layar publik: konfigurasi oleh owner (/queues/:id/display) dan layar tanpa login
// (/display/:token). Opsional; tanpa ini rutenya tidak ada.
func (a *AppRouter) WithDisplay(h DisplayHandler) *AppRouter {
	// simpan handler layar publik, lalu kembalikan router yang sama
	a.displayHandler = h
	return a
}

// WithSubscription memasang endpoint tambah 30 hari langganan owner (opsional; tanpa ini rute /owners tidak ada).
func (a *AppRouter) WithSubscription(h SubscriptionHandler) *AppRouter {
	// simpan handler langganan, lalu kembalikan router yang sama
	a.subscriptionHandler = h
	return a
}

// WithPush memasang endpoint Web Push (opsional; tanpa ini rute /push tidak ada).
func (a *AppRouter) WithPush(h PushHandler) *AppRouter {
	// simpan handler push, lalu kembalikan router yang sama
	a.pushHandler = h
	return a
}

// SetupRouter membuat gin.Engine dan mendaftarkan semua rute sesuai handler yang sudah dipasang di router.
func SetupRouter(
	router *AppRouter,
) *gin.Engine {
	// pakai logger dari AppRouter; bila tidak dipasang, pakai logger default slog (diatur di main)
	logger := router.logger
	if logger == nil {
		logger = slog.Default()
	}

	// mode release mematikan baris "[GIN-debug]" berformat teks agar semua keluaran stdout berupa JSON
	gin.SetMode(gin.ReleaseMode)
	// engine kosong tanpa middleware bawaan: gin.Default() mencatat log teks yang memuat nilai parameter route
	// (termasuk token layar publik), jadi diganti middleware slog di bawah
	engine := gin.New()
	// RequestID paling awal agar semua log request memuat request_id
	engine.Use(middleware.RequestID())
	// Logger sebelum Recovery: request yang panic tetap tercatat sebagai baris akses berstatus 500
	engine.Use(middleware.Logger(logger))
	// Recovery mengubah panic menjadi 500 dan mencatatnya lewat slog
	engine.Use(middleware.Recovery(logger))
	// pasang CORS untuk semua rute, sesuai daftar origin yang diizinkan
	engine.Use(middleware.CORS(router.allowedOrigins))

	// Liveness untuk healthcheck kontainer: tidak menyentuh database atau logika antrian.
	// rute /health langsung membalas status ok
	engine.GET("/health",
		func(c *gin.Context) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status": "ok",
				},
			)
		},
	)

	// semua API berada di bawah /api/v1
	api := engine.Group("/api")
	v1 := api.Group("/v1")

	// register, login, reset password, dan verifikasi email dilayani auth-service (nginx mengarahkan path-nya ke sana),
	// jadi tidak ada rute publik di bawah /users di backend. Backend hanya memverifikasi token yang diterbitkannya.
	// Tiket saya: butuh login, tidak ada di bawah /queues karena tidak terkait satu antrian.
	me := v1.Group("/users/me")
	// semua rute di grup "me" harus lewat pemeriksaan login
	me.Use(router.authMiddleware)
	// rute profil user; hanya didaftarkan bila userHandler ada
	if router.userHandler != nil {
		me.GET("", router.userHandler.Me)
		me.PUT("", router.userHandler.UpdateMe)
		me.PUT("/preferences", router.userHandler.UpdatePreferences)
		me.PUT("/avatar", router.userHandler.UploadAvatar)
		me.GET("/avatar", router.userHandler.GetAvatar)
		me.DELETE("/avatar", router.userHandler.DeleteAvatar)
	}
	// daftar tiket milik user; hanya didaftarkan bila ticketHandler ada
	if router.ticketHandler != nil {
		me.GET("/tickets", router.ticketHandler.MyTickets)
	}
	// riwayat tiket milik user; hanya didaftarkan bila historyHandler ada
	if router.historyHandler != nil {
		me.GET("/history", router.historyHandler.ListMine)
	}

	// semua rute antrian berada di bawah /queues dan wajib login
	queues := v1.Group("/queues")
	queues.Use(router.authMiddleware)
	// rute antrian dan tiket (tanpa id tiket di URL, lihat queue_routes.go)
	registerQueueRoutes(
		queues,
		router.queueHandler,
		router.ticketHandler,
	)
	// koneksi SSE antrian (dilewati otomatis bila sseHandler nil)
	registerSSERoutes(
		queues,
		router.sseHandler,
	)
	// catatan tiket; hanya didaftarkan bila dipasang
	if router.noteHandler != nil {
		registerNoteRoutes(queues, router.noteHandler)
	}
	// CRUD counter; hanya didaftarkan bila dipasang
	if router.counterHandler != nil {
		registerCounterRoutes(queues, router.counterHandler)
	}
	// layar publik: konfigurasi oleh owner, hanya didaftarkan bila dipasang
	if router.displayHandler != nil {
		queues.GET("/:queue_id/display", router.displayHandler.Get)
		queues.PUT("/:queue_id/display", router.displayHandler.Update)
		queues.POST("/:queue_id/display/rotate", router.displayHandler.Rotate)
		queues.DELETE("/:queue_id/display", router.displayHandler.Disable)

		// Layar publik: TANPA login. Satu-satunya kredensial adalah token acak di path.
		display := v1.Group("/display")
		display.GET("/:token", router.displayHandler.Snapshot)
		display.GET("/:token/event", router.displayHandler.Events)
	}

	// riwayat antrian owner; hanya didaftarkan bila dipasang, dan wajib login
	if router.historyHandler != nil {
		history := v1.Group("/history")
		history.Use(router.authMiddleware)
		registerHistoryRoutes(history, router.historyHandler)
	}

	// langganan owner: tambah 30 hari tanpa pembayaran; hanya didaftarkan bila dipasang, dan wajib login
	if router.subscriptionHandler != nil {
		owners := v1.Group("/owners")
		owners.Use(router.authMiddleware)
		owners.POST("/subscription/extend", router.subscriptionHandler.Extend)
	}

	// Web Push; hanya didaftarkan bila dipasang, dan wajib login
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
	// rute join hanya didaftarkan bila ticketHandler ada
	if router.ticketHandler != nil {
		join.GET("/preview", router.ticketHandler.Preview)
		join.POST("", router.ticketHandler.Join)
	}

	// semua rute sudah terdaftar; kembalikan engine untuk dijalankan server
	return engine
}
