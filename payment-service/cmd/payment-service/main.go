// Command payment-service: layanan pembayaran BorderPay. Membuat checkout (internal) dan menerima webhook.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	// import kosong: mendaftarkan driver "pgx" ke database/sql.
	_ "github.com/jackc/pgx/v5/stdlib"

	"qms/payment-service/internal/borderpay"
	"qms/payment-service/internal/checkout"
	"qms/payment-service/internal/config"
	"qms/payment-service/internal/store"
	"qms/payment-service/internal/webhook"
)

func main() {
	// Konfigurasi tidak lengkap: hentikan layanan dengan pesan yang jelas.
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sqlx.Connect("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("koneksi database gagal: %v", err)
	}
	defer db.Close()
	pg := store.NewPostgres(db)

	client, err := borderpay.New(cfg.BorderPayBaseURL, cfg.BorderPayAPIKey, cfg.HTTPTimeout)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	// Health: layanan dianggap hidup bila database bisa dihubungi.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pg.Ping(ctx); err != nil {
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// Checkout internal: hanya backend utama (X-Service-Token).
	mux.Handle("POST /internal/v1/payments", checkout.NewHandler(cfg.ServiceToken, client, pg))
	// Webhook publik: diverifikasi dengan x-borderpay-token di dalam handler.
	mux.Handle("POST /api/v1/webhooks/borderpay", webhook.NewHandler(cfg.WebhookToken, pg))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// Hentikan dengan rapi saat menerima SIGINT/SIGTERM.
	go func() {
		log.Printf("payment-service mendengarkan di :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
