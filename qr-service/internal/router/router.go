package router

import (
	"log/slog"
	"net/http"

	"qms/qr-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

// maxRequestBodyBytes: batas ukuran body request internal (16 KB).
const maxRequestBodyBytes int64 = 16 << 10

// NewAppRouter membuat AppRouter dengan handler wajib: probe dan token, beserta token internal dan logger.
// Fitur opsional dipasang setelahnya lewat metode With...
func NewAppRouter(
	healthHandler HealthHandler,
	tokenHandler TokenHandler,
	serviceToken string,
	logger *slog.Logger,
) *AppRouter {
	// isi AppRouter dengan handler dan konfigurasi wajib
	app := &AppRouter{
		healthHandler: healthHandler,
		tokenHandler:  tokenHandler,
		serviceToken:  serviceToken,
		logger:        logger,
	}
	// kembalikan router yang siap dipasang fitur opsional
	return app
}

// WithRender memasang endpoint render QR umum (opsional; tanpa ini rute /internal/v1/qr/render tidak ada).
func (a *AppRouter) WithRender(h RenderHandler) *AppRouter {
	// simpan handler render, lalu kembalikan router yang sama agar bisa dirangkai
	a.renderHandler = h
	return a
}

// SetupRouter membuat gin.Engine dan mendaftarkan semua rute sesuai handler yang sudah dipasang di router.
//
// Publik: /health, /ready
// Internal: /internal/v1/* (wajib X-Service-Token, tidak diproksikan nginx)
func SetupRouter(
	router *AppRouter,
) *gin.Engine {
	// pakai logger dari AppRouter; bila tidak diisi, pakai logger default slog
	logger := router.logger
	// logger kosong dipakai bila AppRouter dibuat tanpa logger, supaya middleware tidak panic
	if logger == nil {
		logger = slog.Default()
	}

	// mode release menonaktifkan log debug bawaan Gin, lalu buat engine kosong tanpa middleware
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	// pasang middleware global: tangkap panic, beri request ID, catat request, dan larang cache
	engine.Use(gin.Recovery())
	// RequestID dipasang sebelum Logger agar log memuat request_id
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger(logger))
	// respons bisa berisi token, jadi tidak boleh disimpan cache
	engine.Use(middleware.NoStore())

	// rute publik untuk probe liveness dan readiness
	engine.GET("/health", router.healthHandler.Health)
	engine.GET("/ready", router.healthHandler.Ready)

	// grup internal: batasi ukuran body dan wajibkan X-Service-Token sebelum handler dijalankan
	internal := engine.Group("/internal/v1")
	// BodyLimit lebih dulu agar body besar ditolak sebelum diproses
	internal.Use(
		middleware.BodyLimit(maxRequestBodyBytes),
		middleware.ServiceToken(router.serviceToken),
	)
	// rute token join (lihat token_routes.go)
	registerTokenRoutes(
		internal,
		router.tokenHandler,
	)
	// render QR umum; hanya didaftarkan bila dipasang
	if router.renderHandler != nil {
		// rute opsional /internal/v1/qr/render
		registerRenderRoutes(internal, router.renderHandler)
	}

	// rute yang tidak terdaftar: balas 404 dalam format error yang seragam
	engine.NoRoute(func(c *gin.Context) {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "route not found",
				},
			},
		)
	})

	// semua rute sudah terdaftar; kembalikan engine untuk dijalankan server
	return engine
}
