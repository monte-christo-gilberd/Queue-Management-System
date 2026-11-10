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

// main menyusun semua dependensi, menjalankan server HTTP, lalu menunggu sinyal berhenti.
func main() {
	// Muat berkas .env di folder akar proyek (../.env, relatif terhadap folder qr-service/). Di container tidak ada berkas ini,
	// dan semua nilai datang dari environment.
	if err := godotenv.Load("../.env"); err != nil {
		// .env hanya untuk dev; di container semua variabel datang dari environment.
		slog.Debug("no .env file found, relying on process environment")
	}

	// Baca dan validasi konfigurasi. Bila tidak valid, hentikan proses agar container gagal start cepat.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	// Buat logger sesuai LOG_LEVEL lalu jadikan default untuk slog di seluruh proses.
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	// Buat signer dari JOIN_LINK_SECRET. Secret yang terlalu pendek ditolak di sini.
	signer, err := joinlink.New(cfg.JoinSecret)
	if err != nil {
		logger.Error("invalid join link secret", "error", err)
		os.Exit(1)
	}

	// Pasang router dengan signer, TTL dinamis, token internal, dan logger.
	router := httpapi.NewRouter(httpapi.Config{
		Signer:       signer,
		DynamicTTL:   cfg.DynamicTTL,
		ServiceToken: cfg.ServiceToken,
		Logger:       logger,
	})

	// Bungkus router dengan server HTTP yang punya timeout, lalu siapkan context yang batal saat SIGINT/SIGTERM.
	server := httpapi.NewServer(":"+cfg.Port, router)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Jalankan server di goroutine terpisah agar main bisa menunggu sinyal berhenti.
	go func() {
		// Catat port dan TTL dinamis saat server mulai mendengarkan.
		logger.Info("qr-service listening",
			"port", cfg.Port,
			"dynamic_ttl_seconds", int(cfg.DynamicTTL.Seconds()),
		)
		// Server berhenti karena error (bukan karena Shutdown): catat, lalu batalkan ctx agar proses ikut berhenti.
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
			stop()
		}
	}()

	// Tunggu sampai ada sinyal berhenti atau server gagal.
	<-ctx.Done()
	logger.Info("shutting down qr-service")

	// Beri server waktu maksimal 10 detik untuk menyelesaikan request yang sedang berjalan.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Shutdown gagal (mis. timeout): catat, lalu proses tetap berhenti.
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

// newLogger membuat logger JSON ke stdout dengan level dari LOG_LEVEL. Nilai tidak dikenal memakai info.
func newLogger(level string) *slog.Logger {
	// Level log yang dipakai handler. Bawaannya info.
	var lvl slog.Level
	// Terjemahkan nama level dari env (tanpa memperhatikan huruf besar/kecil) ke slog.Level.
	switch strings.ToLower(level) {
	// debug: semua log, termasuk yang tidak penting.
	case "debug":
		lvl = slog.LevelDebug
	// warn: hanya peringatan dan galat.
	case "warn":
		lvl = slog.LevelWarn
	// error: hanya galat.
	case "error":
		lvl = slog.LevelError
	// Nilai lain (termasuk info): info ke atas.
	default:
		lvl = slog.LevelInfo
	}
	// Buat handler JSON ke stdout dengan level yang sudah dipilih.
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
