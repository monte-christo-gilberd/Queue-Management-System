package main

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// TestRunStopsWhenContextCancelled runs run in-process. The database is unreachable, so the listener keeps
// retrying and /health reports 503. Cancelling the context stops run through its shutdown path.
//
// Test ini memakai context, bukan sinyal OS, sehingga bisa jalan di Windows maupun Linux. Pengiriman SIGINT
// sendiri hanya mungkin di Unix (syscall.Kill tidak ada di Windows).
func TestRunStopsWhenContextCancelled(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://qms:qms@127.0.0.1:1/qms?sslmode=disable")
	t.Setenv("VAPID_PUBLIC_KEY", "pub")
	t.Setenv("VAPID_PRIVATE_KEY", "priv")
	t.Setenv("VAPID_SUBJECT", "mailto:test@example.com")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- run(ctx)
	}()

	// The health server starts after the context is set up, so once it answers, cancelling is safe to do.
	var status int
	deadline := time.Now().Add(10 * time.Second)
	for {
		resp, err := http.Get("http://127.0.0.1:8091/health")
		if err == nil {
			status = resp.StatusCode
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("health server did not start: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status != http.StatusServiceUnavailable {
		t.Fatalf("health=%d, want 503 while not listening", status)
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run returned error: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("run did not stop after context cancel")
	}
}
