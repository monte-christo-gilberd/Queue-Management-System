package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"qms/qr-service/internal/joinlink"
)

const maxRequestBodyBytes int64 = 16 << 10 // 16 KB

// Config merangkai dependensi router. Dipisahkan dari config env agar tes dapat
// memasang signer dan TTL sendiri.
type Config struct {
	Signer       *joinlink.Signer
	PublicAppURL string
	DynamicTTL   time.Duration
	ServiceToken string
	Logger       *slog.Logger
}

// NewRouter membangun router publik dan internal.
//
// Publik: /health, /ready, /qr/join.png
// Internal: /internal/v1/* (wajib X-Service-Token, tidak diproksikan nginx)
func NewRouter(cfg Config) *gin.Engine {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(RequestID())
	engine.Use(Logger(logger))
	engine.Use(NoStore())

	handlers := NewHandlers(cfg.Signer, cfg.PublicAppURL, cfg.DynamicTTL)

	engine.GET("/health", handlers.Health)
	engine.GET("/ready", handlers.Ready)
	engine.GET("/qr/join.png", handlers.JoinPNG)

	internal := engine.Group("/internal/v1")
	internal.Use(BodyLimit(maxRequestBodyBytes), ServiceToken(cfg.ServiceToken))
	internal.POST("/join-token", handlers.IssueToken)
	internal.POST("/join-token/verify", handlers.VerifyToken)
	internal.POST("/qr/render", handlers.RenderGeneric)

	engine.NoRoute(func(c *gin.Context) {
		abortWithError(c, http.StatusNotFound, "NOT_FOUND", "route not found")
	})

	return engine
}

// NewServer membungkus router dengan timeout yang wajar agar koneksi lambat
// tidak menggantung selamanya.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
