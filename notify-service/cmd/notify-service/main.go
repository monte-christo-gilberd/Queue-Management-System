package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"qms/notify-service/internal/config"
	"qms/notify-service/internal/event"
	"qms/notify-service/internal/notify"
	"qms/notify-service/internal/push"
	"qms/notify-service/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("db pool", err)
	}
	defer pool.Close()

	// /health: 200 hanya bila LISTEN queue_events sedang tersambung.
	var listening atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		if !listening.Load() {
			http.Error(w, "not listening", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	srv := &http.Server{Addr: ":8091", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server", "err", err)
		}
	}()

	svc := notify.NewService(
		store.New(pool),
		push.New(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject),
	)

	var inflight sync.WaitGroup
	handle := func(e event.Event) {
		inflight.Add(1)
		go func() {
			defer inflight.Done()
			sendCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := svc.Handle(sendCtx, e); err != nil {
				slog.Error("handle event", "kind", e.Kind, "ticket_id", e.TicketID, "err", err)
			}
		}()
	}

	// Koneksi putus: sambung ulang. Sinyal selama putus hilang (outbox menyusul).
	for ctx.Err() == nil {
		err := event.Listen(ctx, cfg.DatabaseURL, handle, func(up bool) { listening.Store(up) })
		listening.Store(false)
		if err != nil && ctx.Err() == nil {
			slog.Warn("listener stopped, reconnecting in 3s", "err", err)
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	}

	slog.Info("shutting down, waiting for in-flight pushes")
	done := make(chan struct{})
	go func() { inflight.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		slog.Warn("shutdown timeout, dropping in-flight pushes")
	}
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
}
