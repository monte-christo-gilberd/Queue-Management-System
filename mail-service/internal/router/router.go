package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"qms/mail-service/internal/middleware"
)

// AppRouter menyimpan handler dan middleware yang akan didaftarkan oleh SetupRouter.
type AppRouter struct {
	// mailHandler: handler kirim email.
	mailHandler MailHandler
	// authMiddleware: pemeriksaan X-Service-Token untuk rute yang dilindungi.
	authMiddleware gin.HandlerFunc
	// systemHandler: handler health check dan 404 (opsional).
	systemHandler SystemHandler
	// logger: pencatat log akses dan panic (opsional; nil berarti slog.Default()).
	logger *slog.Logger
}

// NewAppRouter membuat AppRouter dengan handler wajib: kirim email dan middleware token layanan.
// Fitur opsional dipasang setelahnya lewat metode With...
func NewAppRouter(
	mailHandler MailHandler,
	authMiddleware gin.HandlerFunc,
) *AppRouter {
	// isi AppRouter dengan handler dan middleware wajib
	app := &AppRouter{
		mailHandler:    mailHandler,
		authMiddleware: authMiddleware,
	}
	// kembalikan router yang siap dipasang fitur opsional
	return app
}

// WithSystem memasang rute GET /health dan balasan 404 untuk rute lain (opsional; tanpa ini keduanya tidak ada).
func (a *AppRouter) WithSystem(
	h SystemHandler,
) *AppRouter {
	// simpan handler sistem, lalu kembalikan router yang sama agar bisa dirangkai
	a.systemHandler = h
	// Kembalikan router yang sama agar pemanggilan bisa dirangkai.
	return a
}

// WithLogger memasang logger untuk log akses dan Recovery (opsional; tanpa ini dipakai slog.Default()).
func (a *AppRouter) WithLogger(l *slog.Logger) *AppRouter {
	// simpan logger, lalu kembalikan router yang sama agar bisa dirangkai
	a.logger = l
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

	// mode release menonaktifkan log debug bawaan Gin, lalu buat engine kosong tanpa middleware bawaan
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	// RequestID paling awal agar semua log request memuat request_id
	engine.Use(middleware.RequestID())
	// Logger sebelum Recovery: request yang panic tetap tercatat sebagai baris akses berstatus 500
	engine.Use(middleware.Logger(logger))
	// Recovery mengubah panic menjadi 500 dan mencatatnya lewat slog
	engine.Use(middleware.Recovery(logger))
	// Service ini hanya dipanggil lewat jaringan internal; header X-Forwarded-For tidak dipakai.
	_ = engine.SetTrustedProxies(nil)
	// rute sistem hanya didaftarkan bila dipasang
	if router.systemHandler != nil {
		// Health check: selalu 200 selama proses hidup.
		engine.GET("/health", router.systemHandler.Health)
		// rute lain (termasuk metode yang salah): balas 404 dengan format error yang seragam
		engine.NoRoute(router.systemHandler.NotFound)
	}
	// rute kirim email hanya didaftarkan bila handler ada
	if router.mailHandler != nil {
		registerMailRoutes(
			engine,
			router.mailHandler,
			router.authMiddleware,
		)
	}
	// semua rute sudah terdaftar; kembalikan engine untuk dijalankan server
	return engine
}
