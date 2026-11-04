// payment-service: memperpanjang langganan satu bulan setelah pembayaran Stripe berhasil; pembayaran pertama
// sebuah user juga menjadikannya owner.
// Sengaja terpisah dari backend: backend hanya MEMBACA owners.subscription_expires_at dan tidak pernah
// memanggil layanan ini, jadi bila payment-service mati antrian tetap berjalan; hanya perpanjangan yang tertunda.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"qms/payment-service/internal/billing"
	"qms/payment-service/internal/config"
	"qms/payment-service/internal/httpapi"
)

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// pgxpool.New tidak langsung menyambung: database yang belum siap tidak membuat service gagal start,
	// permintaan baru berhasil setelah database pulih.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("database config", err)
	}
	defer pool.Close()

	var provider billing.Provider
	if cfg.DevSimulator {
		slog.Warn("DEV_PAYMENT_SIMULATOR is ON: payments are FAKE (no Stripe, no money). Never enable in production.")
	} else if cfg.StripeConfigured() {
		provider = billing.NewStripeProvider(cfg.StripeKey, cfg.StripeBaseURL, cfg.WebhookSecret, cfg.PriceID, cfg.Amount, cfg.Currency)
	} else {
		// Tidak fatal: service tetap hidup supaya salah konfigurasi di sini tidak mengganggu yang lain.
		slog.Error("STRIPE_API_KEY / STRIPE_WEBHOOK_SECRET not configured: payments disabled, checkout returns 503")
	}
	svc := billing.New(billing.NewPGStore(pool), provider, cfg.PublicOrigin, slog.Default())
	if cfg.DevSimulator {
		svc.WithSimulator(cfg.Amount, cfg.Currency)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.New(svc, cfg.JWTSecret, slog.Default()).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		slog.Info("payment-service starting", "port", cfg.Port, "enabled", svc.Enabled(), "simulator", cfg.DevSimulator, "stripe_base_url", cfg.StripeBaseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("server", err)
		}
	}()

	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
