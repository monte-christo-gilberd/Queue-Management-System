package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"qms/qr-service/internal/joinlink"
)

// maxRequestBodyBytes: batas ukuran body request internal (16 KB).
const maxRequestBodyBytes int64 = 16 << 10 // 16 KB

// Config merangkai dependensi router. Dipisahkan dari config env agar tes dapat
// memasang signer dan TTL sendiri.
type Config struct {
	// Signer: pembuat dan verifikator token join.
	Signer *joinlink.Signer
	// DynamicTTL: masa berlaku token dinamis yang dibuat endpoint IssueToken bila tidak diatur klien.
	DynamicTTL time.Duration
	// ServiceToken: nilai X-Service-Token yang wajib dibawa request internal.
	ServiceToken string
	// Logger: pencatat request. Nil = slog default.
	Logger *slog.Logger
}

// NewRouter membangun router publik dan internal.
//
// Publik: /health, /ready
// Internal: /internal/v1/* (wajib X-Service-Token, tidak diproksikan nginx)
func NewRouter(cfg Config) *gin.Engine {
	// Pakai logger dari konfigurasi; bila tidak diisi, pakai logger default slog.
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Mode release menonaktifkan log debug bawaan Gin, lalu buat engine kosong tanpa middleware.
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	// Pasang middleware global: tangkap panic, beri request ID, catat request, dan larang cache.
	engine.Use(gin.Recovery())
	engine.Use(RequestID())
	engine.Use(Logger(logger))
	engine.Use(NoStore())

	// Buat handler dengan signer dan TTL dinamis dari konfigurasi.
	handlers := NewHandlers(cfg.Signer, cfg.DynamicTTL)

	// Rute publik untuk probe liveness dan readiness.
	engine.GET("/health", handlers.Health)
	engine.GET("/ready", handlers.Ready)

	// Grup internal: batasi ukuran body dan wajibkan X-Service-Token sebelum handler dijalankan.
	internal := engine.Group("/internal/v1")
	internal.Use(BodyLimit(maxRequestBodyBytes), ServiceToken(cfg.ServiceToken))
	// Buat token join baru.
	internal.POST("/join-token", handlers.IssueToken)
	// Periksa tanda tangan dan masa berlaku token.
	internal.POST("/join-token/verify", handlers.VerifyToken)
	// Render QR umum menjadi PNG.
	internal.POST("/qr/render", handlers.RenderGeneric)

	// Rute yang tidak terdaftar: balas 404 dalam format error yang seragam.
	engine.NoRoute(func(c *gin.Context) {
		abortWithError(c, http.StatusNotFound, "NOT_FOUND", "route not found")
	})

	return engine
}

// NewServer membungkus router dengan timeout yang wajar agar koneksi lambat
// tidak menggantung selamanya.
func NewServer(addr string, handler http.Handler) *http.Server {
	// Susun server dengan alamat, handler, dan batas waktu baca, tulis, dan koneksi diam.
	return &http.Server{
		// Alamat dan port tempat server mendengarkan.
		Addr: addr,
		// Router yang menangani semua request.
		Handler: handler,
		// Batas waktu membaca header request (5 detik), melawan klien yang lambat.
		ReadHeaderTimeout: 5 * time.Second,
		// Batas waktu membaca seluruh request (10 detik).
		ReadTimeout: 10 * time.Second,
		// Batas waktu menulis respons (15 detik).
		WriteTimeout: 15 * time.Second,
		// Batas koneksi keep-alive yang diam (60 detik).
		IdleTimeout: 60 * time.Second,
	}
}
