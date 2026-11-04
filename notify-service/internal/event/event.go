// Package event mengurai payload NOTIFY dari trigger tickets_notify dan mendengarkannya.
package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

const (
	KindCalled   = "called"
	KindRecalled = "recalled"
	KindSkipped  = "skipped"
	// KindRequeued: nomor kembali menunggu (skip dengan fallback, atau counter dihapus).
	KindRequeued = "requeued"
)

const (
	ReasonSkip          = "skip"
	ReasonCounterClosed = "counter_closed"
)

type Event struct {
	Kind     string `json:"kind"`
	QueueID  int64  `json:"queue_id"`
	TicketID int64  `json:"ticket_id"`
	// Reason dan Counter hanya terisi untuk requeued. Counter dibawa di payload karena counter yang
	// dihapus sudah tidak bisa dibaca dari database.
	Reason  string `json:"reason,omitempty"`
	Counter string `json:"counter,omitempty"`
}

func Parse(raw string) (Event, error) {
	var e Event
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return Event{}, err
	}
	if e.Kind == "" || e.QueueID == 0 || e.TicketID == 0 {
		return Event{}, fmt.Errorf("incomplete event: %q", raw)
	}
	return e, nil
}

// Listen berjalan sampai koneksi putus atau ctx selesai. LISTEN hanya jalan di primary.
// onState dipanggil true setelah LISTEN aktif (untuk healthcheck); boleh nil.
func Listen(ctx context.Context, dsn string, handle func(Event), onState func(bool)) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN queue_events"); err != nil {
		return err
	}
	slog.Info("listening on queue_events")
	if onState != nil {
		onState(true)
	}
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		e, err := Parse(n.Payload)
		if err != nil {
			slog.Warn("skip bad event", "err", err)
			continue
		}
		handle(e)
	}
}
