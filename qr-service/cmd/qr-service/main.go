// Command qr-service menjalankan microservice QR QMS: pembuat/verifikator token
// join dan perender PNG QR. Service ini stateless dan tidak mengakses database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"qms/qr-service/internal/config"
	"qms/qr-service/internal/handler"
	"qms/qr-service/internal/httpserver"
	"qms/qr-service/internal/logging"
	"qms/qr-service/internal/metrics"
	"qms/qr-service/internal/router"
	"qms/qr-service/internal/service"
)

// main hanya memanggil run, lalu menghentikan proses dengan kode 1 bila run gagal.
func main() {
	// jalankan service; galat dicatat lalu proses berhenti agar container gagal start cepat
	if err := run(); err != nil {
		// catat penyebab, lalu keluar dengan kode 1 agar orkestrator tahu start gagal
		slog.Error("qr-service stopped", "error", err)
		os.Exit(1)
	}
}

// run menyusun semua dependensi, menjalankan server HTTP, lalu menunggu sinyal berhenti.
func run() error {
	// muat berkas .env di folder akar proyek (../.env, relatif terhadap folder qr-service/); di container
	// tidak ada berkas ini, dan semua nilai datang dari environment
	if err := godotenv.Load("../.env"); err != nil {
		// .env hanya untuk dev; di container semua variabel datang dari environment
		slog.Debug("no .env file found, relying on process environment")
	}

	// baca dan validasi konfigurasi; bila tidak valid, kembalikan galat
	cfg, err := config.Load()
	// konfigurasi tidak valid (secret kosong, TTL di luar rentang): berhenti sebelum membuka port
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// buat logger sesuai LOG_LEVEL lalu jadikan default untuk slog di seluruh proses
	logger := logging.NewLogger("qr-service", cfg.LogLevel)
	slog.SetDefault(logger)

	// buat service token dari JOIN_LINK_SECRET; secret yang terlalu pendek ditolak di sini
	tokenService, err := service.NewTokenService(cfg.JoinSecret, cfg.DynamicTTL)
	// service memeriksa ulang panjang secret: berhenti sebelum membuka port
	if err != nil {
		return fmt.Errorf("invalid join link secret: %w", err)
	}
	// buat service render QR
	renderService := service.NewRenderService()

	// buat handler dari service
	healthHandler := handler.NewHealthHandler()
	// handler token dan render memakai service masing-masing
	tokenHandler := handler.NewTokenHandler(tokenService)
	renderHandler := handler.NewRenderHandler(renderService)

	// pasang router dengan handler wajib, token internal, dan logger, lalu fitur opsional
	appRouter := router.NewAppRouter(
		healthHandler,
		tokenHandler,
		cfg.ServiceToken,
		logger,
	).WithRender(renderHandler)
	// metrik Prometheus: middleware HTTP dipasang di router, listener /metrics dijalankan setelah ctx dibuat
	appMetrics := metrics.NewQR()
	appRouter.WithMetrics(appMetrics.Middleware())
	// bangun gin.Engine dengan semua rute dan middleware terpasang
	engine := router.SetupRouter(appRouter)

	// bungkus router dengan server HTTP yang punya timeout
	server := httpserver.NewServer(":"+cfg.Port, engine)

	// siapkan context yang batal saat SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	// stop melepas penangkap sinyal saat run selesai
	defer stop()
	// listener metrik di port terpisah (METRICS_ADDR, bawaan :9100); tidak memblokir dan tidak menggagalkan service
	appMetrics.Serve(ctx)

	// jalankan server di goroutine terpisah agar run bisa menunggu sinyal berhenti
	go func() {
		// catat port dan TTL dinamis saat server mulai mendengarkan
		logger.Info("qr-service listening",
			"port", cfg.Port,
			"dynamic_ttl_seconds", int(cfg.DynamicTTL.Seconds()),
		)
		// server berhenti karena error (bukan karena Shutdown): catat, lalu batalkan ctx agar proses ikut berhenti
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			stop()
		}
	}()

	// tunggu sampai ada sinyal berhenti atau server gagal
	<-ctx.Done()
	// catat bahwa sinyal berhenti sudah diterima
	logger.Info("shutting down qr-service")

	// beri server waktu maksimal 10 detik untuk menyelesaikan request yang sedang berjalan
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// cancel melepas timer context shutdown saat run selesai
	defer cancel()
	// shutdown gagal (mis. timeout): catat, lalu proses tetap berhenti
	if err := server.Shutdown(shutdownCtx); err != nil {
		// shutdown gagal tidak mengubah hasil run; cukup dicatat
		logger.Error("graceful shutdown failed", "error", err)
	}
	// penutupan normal
	return nil
}
