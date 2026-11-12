package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"qms/auth-service/internal/middleware"
)

// NewAppRouter membuat AppRouter dengan handler wajib: registrasi, login, dan middleware login.
// Fitur opsional dipasang setelahnya lewat metode With...
func NewAppRouter(
	authHandler AuthHandler,
	authMiddleware gin.HandlerFunc,
	trustedProxies []string,
) *AppRouter {
	// isi AppRouter dengan handler dan middleware wajib
	app := &AppRouter{
		authHandler:    authHandler,
		authMiddleware: authMiddleware,
		trustedProxies: trustedProxies,
	}
	// kembalikan router yang siap dipasang fitur opsional
	return app
}

// WithEmail memasang rute reset password dan verifikasi email (opsional; tanpa ini rutenya tidak ada).
func (a *AppRouter) WithEmail(h EmailHandler) *AppRouter {
	// simpan handler email, lalu kembalikan router yang sama agar bisa dirangkai
	a.emailHandler = h
	return a
}

// WithGoogle memasang rute login Google (opsional; tanpa ini rutenya tidak ada).
func (a *AppRouter) WithGoogle(h GoogleHandler) *AppRouter {
	// simpan handler Google, lalu kembalikan router yang sama
	a.googleHandler = h
	return a
}

// WithLogger memasang logger untuk log akses dan Recovery (opsional; tanpa ini dipakai slog.Default()).
func (a *AppRouter) WithLogger(l *slog.Logger) *AppRouter {
	// simpan logger, lalu kembalikan router yang sama agar bisa dirangkai
	a.logger = l
	return a
}

// WithTracing memasang middleware tracing (opsional; tanpa ini tidak ada span request).
func (a *AppRouter) WithTracing(m gin.HandlerFunc) *AppRouter {
	// simpan middleware, lalu kembalikan router yang sama agar bisa dirangkai
	a.tracingMiddleware = m
	return a
}

// WithMetrics memasang middleware metrik HTTP (opsional; tanpa ini tidak ada metrik request).
func (a *AppRouter) WithMetrics(m gin.HandlerFunc) *AppRouter {
	// simpan middleware, lalu kembalikan router yang sama agar bisa dirangkai
	a.metricsMiddleware = m
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
	// gin.New tanpa logger bawaan Gin (teks, memuat nilai parameter route); log akses dicatat middleware slog di bawah
	engine := gin.New()
	// RequestID paling awal agar semua log request memuat request_id (nilainya dari X-Request-Id yang diisi nginx)
	// tracing paling awal: span server mencakup semua middleware di bawahnya, dan span aktif sudah ada saat
	// RequestID, Logger, dan Recovery berjalan (atribut request_id dan trace_id di log)
	if router.tracingMiddleware != nil {
		engine.Use(router.tracingMiddleware)
	}
	engine.Use(middleware.RequestID())
	// metrik sebelum Logger dan Recovery: request yang panic tetap terhitung sebagai 500
	if router.metricsMiddleware != nil {
		engine.Use(router.metricsMiddleware)
	}
	// Logger sebelum Recovery: request yang panic tetap tercatat sebagai baris akses berstatus 500
	engine.Use(middleware.Logger(logger))
	// Recovery mengubah panic di handler menjadi 500 (bukan proses mati) dan mencatatnya lewat slog
	engine.Use(middleware.Recovery(logger))
	// hanya proxy ini yang boleh menentukan IP klien lewat X-Forwarded-For (dipakai pembatas per akun)
	_ = engine.SetTrustedProxies(router.trustedProxies)

	// Liveness: tidak menyentuh database, sehingga gangguan DB tidak membuat layanan dianggap mati.
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

	// semua rute akun berada di bawah /api/v1/users
	users := engine.Group("/api/v1/users")
	// register dan login selalu ada
	registerAuthRoutes(
		users,
		router.authHandler,
	)
	// login Google hanya dipasang bila handler Google ada; selain itu rutenya tidak ada (404)
	if router.googleHandler != nil {
		registerGoogleRoutes(
			users,
			router.googleHandler,
		)
	}
	// reset password dan verifikasi email hanya dipasang bila handler email ada; selain itu 404
	if router.emailHandler != nil {
		registerEmailRoutes(
			users,
			router.emailHandler,
		)
		// kirim ulang verifikasi butuh login: hanya rute ini di grup "me" (profil dan lainnya tetap di backend)
		// grup "me" membuat grup rute terpisah yang diberi middleware login
		me := engine.Group("/api/v1/users/me")
		// middleware login memastikan hanya user yang login bisa memanggil rute ini
		me.Use(router.authMiddleware)
		// daftarkan rute kirim ulang verifikasi di grup yang sudah diberi middleware
		registerMeRoutes(
			me,
			router.emailHandler,
		)
	}

	// semua rute sudah terdaftar; kembalikan engine untuk dijalankan server
	return engine
}
