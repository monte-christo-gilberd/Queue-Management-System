// mail-service: satu-satunya pengirim email di QMS. Stateless dan tanpa database: menerima permintaan lewat
// API internal bertoken, mengisi templat, dan mengirim lewat SMTP (Mailpit di dev, SMTP nyata di produksi).
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

	"qms/mail-service/internal/config"
	"qms/mail-service/internal/httpapi"
	"qms/mail-service/internal/mailer"
	"qms/mail-service/internal/outbox"

	"github.com/jackc/pgx/v5/pgxpool"
)

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

// sendTest: `mail-service send-test <alamat>` mengirim satu email uji lewat SMTP yang dikonfigurasi lalu keluar.
// Hanya butuh SMTP_*; dipakai memeriksa jalur email tanpa lewat API.
func sendTest(to string) {
	smtpCfg, err := config.LoadSMTP()
	if err != nil {
		fatal("config", err)
	}
	r, err := mailer.Render("test", map[string]any{
		"To":     to,
		"SentAt": time.Now().Format("2 Jan 2006 15:04:05 MST"),
		"Link":   "http://localhost:8080/reset-password?token=contoh-token-uji",
	})
	if err != nil {
		fatal("render", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := mailer.New(smtpCfg).Send(ctx, mailer.Message{To: to, Subject: r.Subject, Text: r.Text, HTML: r.HTML}); err != nil {
		fatal("send test email", err)
	}
	slog.Info("test email sent", "smtp", smtpCfg.Addr())
}

// runOutbox menjalankan worker outbox. Kegagalan menyambung database tidak mematikan layanan (API kirim tetap
// jalan); worker dicoba lagi sampai berhasil.
func runOutbox(ctx context.Context, cfg config.Config) {
	var pool *pgxpool.Pool
	for pool == nil && ctx.Err() == nil {
		p, err := pgxpool.New(ctx, cfg.Outbox.DatabaseURL)
		if err == nil {
			err = p.Ping(ctx)
		}
		if err != nil {
			slog.Error("outbox database not reachable, retrying", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		pool = p
	}
	if pool == nil {
		return
	}
	defer pool.Close()
	w := outbox.NewWorker(outbox.NewPG(pool), mailer.New(cfg.SMTP), outbox.Config{
		PublicOrigin: cfg.Outbox.PublicOrigin,
		Rate:         cfg.Outbox.Rate,
		MaxAttempts:  cfg.Outbox.MaxAttempts,
	}, slog.Default())
	go outbox.Listen(ctx, pool, w.Wake, slog.Default())
	slog.Info("outbox worker started", "rate_per_sec", cfg.Outbox.Rate, "max_attempts", cfg.Outbox.MaxAttempts)
	w.Run(ctx)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if len(os.Args) > 1 && os.Args[1] == "send-test" {
		if len(os.Args) != 3 {
			fatal("usage", errors.New("mail-service send-test <alamat-email>"))
		}
		sendTest(os.Args[2])
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	api := httpapi.New(mailer.New(cfg.SMTP), cfg.ServiceToken, slog.Default())
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second, // pengiriman sinkron dengan coba ulang bisa sampai sekitar 45 detik
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if cfg.Outbox.Enabled() {
		go runOutbox(ctx, cfg)
	} else {
		slog.Warn("DATABASE_URL not set: outbox worker disabled, only POST /internal/v1/send is served")
	}
	go func() {
		slog.Info("mail-service listening", "addr", srv.Addr, "smtp", cfg.SMTP.Addr(), "tls", cfg.SMTP.TLS)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("server", err)
		}
	}()
	<-ctx.Done()

	slog.Info("shutting down, waiting for in-flight emails")
	shCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
}
