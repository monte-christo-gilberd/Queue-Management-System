package main

import (
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

// TestMainRunsUntilSignal runs main in-process. The database is unreachable, so the listener keeps
// retrying and /health reports 503. SIGINT then stops main through its signal context.
func TestMainRunsUntilSignal(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://qms:qms@127.0.0.1:1/qms?sslmode=disable")
	t.Setenv("VAPID_PUBLIC_KEY", "pub")
	t.Setenv("VAPID_PRIVATE_KEY", "priv")
	t.Setenv("VAPID_SUBJECT", "mailto:test@example.com")

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	// The health server starts after the signal context is set up, so once it answers, SIGINT is safe to send.
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

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("main did not stop after SIGINT")
	}
}
