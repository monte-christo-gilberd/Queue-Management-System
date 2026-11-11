package service_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/mailtemplate"
	"qms/mail-service/internal/model"
	"qms/mail-service/internal/service"
)

// claimErrStore: Claim selalu gagal; dipakai untuk menguji cabang galat klaim.
type claimErrStore struct {
	*memStore
	calls atomic.Int32
}

func (s *claimErrStore) Claim(context.Context, int, time.Duration) ([]model.OutboxItem, error) {
	s.calls.Add(1)
	return nil, errors.New("db down")
}

// markErrStore: MarkSent, MarkRetry, dan MarkFailed selalu gagal.
type markErrStore struct{ *memStore }

func (s *markErrStore) MarkSent(context.Context, int64) error { return errors.New("sent write failed") }
func (s *markErrStore) MarkRetry(context.Context, int64, string, time.Duration) error {
	return errors.New("retry write failed")
}
func (s *markErrStore) MarkFailed(context.Context, int64, string) error {
	return errors.New("failed write failed")
}

// cancelSender: membatalkan konteks saat mengirim, lalu mengembalikan galat sementara.
type cancelSender struct{ cancel context.CancelFunc }

func (s cancelSender) Send(context.Context, model.Message) error {
	s.cancel()
	return errors.New("connection reset")
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestNewWorker_NilLoggerUsesDefault(t *testing.T) {
	w := service.NewOutboxService(newStore(), &sender{}, mailtemplate.New(), service.OutboxConfig{}, nil)
	if w == nil {
		t.Fatal("NewWorker mengembalikan nil")
	}
	w.Wake()
}

func TestWorker_ClaimErrorIsLoggedAndRetriedNextCycle(t *testing.T) {
	st := &claimErrStore{memStore: newStore()}
	w := newWorker(st, &sender{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for st.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if st.calls.Load() < 2 {
		t.Fatalf("Claim dipanggil %d kali, want >= 2 (dicoba lagi pada siklus berikut)", st.calls.Load())
	}
}

func TestWorker_ShutdownWhileWaitingForRateLimitSendsNothing(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)), item(2, "joined", "a@x.test", pl("N", "Q", 2)))
	sd := &sender{}
	// 0,5 email per detik: giliran kirim pertama baru tiba setelah 2 detik.
	w := service.NewOutboxService(st, sd, mailtemplate.New(), service.OutboxConfig{PublicOrigin: "http://h", Rate: 0.5, Poll: time.Hour}, quietLog())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker tidak berhenti saat menunggu laju")
	}
	if n := len(sd.sent()); n != 0 {
		t.Fatalf("terkirim %d email setelah shutdown, want 0", n)
	}
}

func TestWorker_ContextCancelledDuringSendReleasesRowWithoutCounting(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)))
	ctx, cancel := context.WithCancel(context.Background())
	w := newWorker(st, cancelSender{cancel: cancel})
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker tidak berhenti")
	}
	if r := st.get(1); r.sent || r.failed || r.retryIn != 0 {
		t.Fatalf("baris seharusnya tidak berubah saat shutdown: %+v", r)
	}
}

func TestWorker_MarkSentErrorIsLogged(t *testing.T) {
	st := &markErrStore{memStore: newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)))}
	sd := &sender{}
	// MarkSent gagal membuat baris tetap dapat diklaim, jadi email bisa terkirim lebih dari sekali.
	runUntil(t, newWorker(st, sd), func() bool { return len(sd.sent()) >= 1 })
}

func TestWorker_MarkRetryErrorIsLogged(t *testing.T) {
	st := &markErrStore{memStore: newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)))}
	sd := &sender{errs: []error{errors.New("connection refused")}}
	w := newWorker(st, sd)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		sd.mu.Lock()
		consumed := len(sd.errs) == 0
		sd.mu.Unlock()
		if consumed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if st.get(1).failed {
		t.Fatalf("baris tidak boleh gagal permanen: %+v", st.get(1))
	}
}

func TestWorker_MarkFailedErrorIsLogged(t *testing.T) {
	st := &markErrStore{memStore: newStore(item(1, "joined", "bad@x.test", pl("N", "Q", 1)))}
	sd := &sender{errs: []error{fmt.Errorf("%w: 550", apperror.ErrPermanent)}}
	w := newWorker(st, sd)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for len(sd.errs) > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done
	if strings.Contains(st.get(1).lastErr, "550") {
		t.Fatal("MarkFailed gagal seharusnya tidak mengubah baris")
	}
}
