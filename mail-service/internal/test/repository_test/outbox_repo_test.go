package repository_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/v5/stdlib"

	"qms/mail-service/internal/repository"
)

// unreachableDSN: alamat yang pasti menolak koneksi.
const unreachableDSN = "postgres://u:p@127.0.0.1:1/none?connect_timeout=1&sslmode=disable"

// unreachablePool membuat pool ke alamat yang pasti menolak koneksi. sqlx.Open tidak menyambung saat dibuat,
// jadi setiap query gagal dengan galat jaringan: cukup untuk menguji jalur galat PG tanpa database.
func unreachablePool(t *testing.T) *sqlx.DB {
	t.Helper()
	db, err := sqlx.Open("pgx", unreachableDSN)
	if err != nil {
		t.Fatalf("sqlx.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPG_QueriesReturnErrorWhenDatabaseUnreachable(t *testing.T) {
	st := repository.NewOutboxRepository(unreachablePool(t))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := st.Claim(ctx, 10, time.Minute); err == nil || !strings.Contains(err.Error(), "claim outbox") {
		t.Errorf("Claim: err = %v, want galat terbungkus 'claim outbox'", err)
	}
	if err := st.MarkSent(ctx, 1); err == nil {
		t.Error("MarkSent: want galat")
	}
	// Penyebab panjang dipotong ke 500 byte sebelum dikirim ke database.
	if err := st.MarkRetry(ctx, 1, strings.Repeat("x", 1000), time.Second); err == nil {
		t.Error("MarkRetry: want galat")
	}
	if err := st.MarkFailed(ctx, 1, strings.Repeat("y", 1000)); err == nil {
		t.Error("MarkFailed: want galat")
	}
	if err := st.MarkFailed(ctx, 1, "pendek"); err == nil {
		t.Error("MarkFailed (penyebab pendek): want galat")
	}
}

func TestPG_ListenReturnsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		repository.Listen(ctx, unreachableDSN, func() { t.Error("wake tidak boleh dipanggil tanpa koneksi") },
			slog.New(slog.NewTextHandler(io.Discard, nil)))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Listen tidak berhenti setelah konteks selesai")
	}
}

func TestPG_ListenStopsImmediatelyWhenAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository.Listen(ctx, unreachableDSN, func() {}, nil)
}
