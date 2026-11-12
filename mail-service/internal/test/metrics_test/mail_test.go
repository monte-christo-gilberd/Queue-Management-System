package metrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"qms/mail-service/internal/metrics"
	"qms/mail-service/internal/model"
	"qms/mail-service/internal/service"
)

// fakeStore memenuhi service.OutboxStore; hanya MarkSent dan MarkFailed yang dipakai tes ini.
type fakeStore struct{ sentErr, failErr error }

func (fakeStore) Claim(context.Context, int, time.Duration) ([]model.OutboxItem, error) {
	return nil, nil
}
func (f fakeStore) MarkSent(context.Context, int64) error                       { return f.sentErr }
func (fakeStore) MarkRetry(context.Context, int64, string, time.Duration) error { return nil }
func (f fakeStore) MarkFailed(context.Context, int64, string) error             { return f.failErr }

// Hasil akhir dihitung hanya bila penandaan di database berhasil.
func TestMeteredStore_CountsOnlySuccessfulMarks(t *testing.T) {
	m := metrics.NewMail()
	ok := service.NewMeteredOutboxStore(fakeStore{}, m)
	bad := service.NewMeteredOutboxStore(fakeStore{sentErr: errors.New("db"), failErr: errors.New("db")}, m)
	ctx := context.Background()

	_ = ok.MarkSent(ctx, 1)
	_ = ok.MarkFailed(ctx, 2, "x")
	_ = bad.MarkSent(ctx, 3)
	_ = bad.MarkFailed(ctx, 4, "x")

	if got := counter(t, m.Registry, "mail_sent_total", nil); got != 1 {
		t.Errorf("mail_sent_total = %v, diharapkan 1", got)
	}
	if got := counter(t, m.Registry, "mail_failed_total", nil); got != 1 {
		t.Errorf("mail_failed_total = %v, diharapkan 1", got)
	}
}

// Gauge pending dihitung saat scrape; galat query membuat sampel dilewati, bukan angka palsu.
func TestPendingGauge(t *testing.T) {
	m := metrics.NewMail()
	n, fail := int64(7), false
	m.SetPending(func(context.Context) (int64, error) {
		if fail {
			return 0, errors.New("db down")
		}
		return n, nil
	})

	g := find(t, m.Registry, "mail_outbox_pending", nil)
	if g == nil || g.GetGauge().GetValue() != 7 {
		t.Fatalf("mail_outbox_pending = %v, diharapkan 7", g)
	}
	fail = true
	// Gather melaporkan galat collector, tetapi metrik lain tetap terkumpul
	if _, err := m.Registry.Gather(); err == nil {
		t.Fatal("galat query seharusnya dilaporkan Gather")
	}
	if g := find(t, m.Registry, "mail_outbox_pending", nil); g != nil {
		t.Fatalf("saat query gagal sampel harus dilewati, dapat %v", g)
	}
}
