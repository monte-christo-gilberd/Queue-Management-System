// Command qr-service menjalankan microservice QR QMS: pembuat/verifikator token
// join dan perender PNG QR. Service ini stateless dan tidak mengakses database.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"qms/qr-service/internal/config"
	"qms/qr-service/internal/httpapi"
	"qms/qr-service/internal/joinlink"
)

func main() {
	if err := godotenv.Load(); err != nil {
		// .env hanya untuk dev; di container semua variabel datang dari environment.
		slog.Debug("no .env file found, relying on process environment")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	signer, err := joinlink.New(cfg.JoinSecret)
	if err != nil {
		logger.Error("invalid join link secret", "error", err)
		os.Exit(1)
	}

	router := httpapi.NewRouter(httpapi.Config{
		Signer:       signer,
		PublicAppURL: cfg.PublicAppURL,
		DynamicTTL:   cfg.DynamicTTL,
		ServiceToken: cfg.ServiceToken,
		Logger:       logger,
	})

	server := httpapi.NewServer(":"+cfg.Port, router)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("qr-service listening",
			"port", cfg.Port,
			"public_app_url", cfg.PublicAppURL,
			"dynamic_ttl_seconds", int(cfg.DynamicTTL.Seconds()),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down qr-service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
