package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"qms/notify-service/internal/config"
	"qms/notify-service/internal/event"
	"qms/notify-service/internal/notify"
	"qms/notify-service/internal/push"
	"qms/notify-service/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	svc := notify.NewService(
		store.New(pool),
		push.New(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject),
	)

	handle := func(e event.Event) {
		go func() {
			sendCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := svc.Handle(sendCtx, e); err != nil {
				log.Printf("handle %s ticket=%d: %v", e.Kind, e.TicketID, err)
			}
		}()
	}

	// Koneksi putus: sambung ulang. Sinyal selama putus hilang (outbox menyusul).
	for ctx.Err() == nil {
		if err := event.Listen(ctx, cfg.DatabaseURL, handle); err != nil && ctx.Err() == nil {
			log.Printf("listener stopped: %v; reconnecting in 3s", err)
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	}
}
