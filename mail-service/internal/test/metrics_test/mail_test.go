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

// fakeStore memenuhi service.OutboxStore; hanya MarkSent dan MarkFailed yang dipakai tes ini, dan hasilnya
// (galat atau tidak) bisa diatur per instance untuk meniru database yang gagal menyimpan.
type fakeStore struct{ sentErr, failErr error }

// Claim tidak dipakai tes ini.
func (fakeStore) Claim(context.Context, int, time.Duration) ([]model.OutboxItem, error) {
	return nil, nil
}

// MarkSent mengembalikan galat yang diatur (nil = berhasil).
func (f fakeStore) MarkSent(context.Context, int64) error { return f.sentErr }

// MarkRetry tidak dipakai tes ini.
func (fakeStore) MarkRetry(context.Context, int64, string, time.Duration) error { return nil }

// MarkFailed mengembalikan galat yang diatur (nil = berhasil).
func (f fakeStore) MarkFailed(context.Context, int64, string) error { return f.failErr }

// Hasil akhir dihitung hanya bila penandaan di database berhasil, supaya angka sama dengan isi tabel outbox.
func TestMeteredStore_CountsOnlySuccessfulMarks(t *testing.T) {
	m := metrics.NewMail()
	// ok: store yang berhasil menyimpan; bad: store yang selalu gagal menyimpan
	ok := service.NewMeteredOutboxStore(fakeStore{}, m)
	bad := service.NewMeteredOutboxStore(fakeStore{sentErr: errors.New("db"), failErr: errors.New("db")}, m)
	ctx := context.Background()

	// dua penandaan berhasil, dua penandaan gagal
	_ = ok.MarkSent(ctx, 1)
	_ = ok.MarkFailed(ctx, 2, "x")
	_ = bad.MarkSent(ctx, 3)
	_ = bad.MarkFailed(ctx, 4, "x")

	// hanya yang berhasil yang terhitung
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
	// n dan fail diubah di antara dua pembacaan untuk meniru database yang sehat lalu gagal
	n, fail := int64(7), false
	m.SetPending(func(context.Context) (int64, error) {
		if fail {
			return 0, errors.New("db down")
		}
		return n, nil
	})

	// saat database sehat: nilai gauge sama dengan hasil hitung
	g := find(t, m.Registry, "mail_outbox_pending", nil)
	if g == nil || g.GetGauge().GetValue() != 7 {
		t.Fatalf("mail_outbox_pending = %v, diharapkan 7", g)
	}
	fail = true
	// Gather melaporkan galat collector, tetapi metrik lain tetap terkumpul
	if _, err := m.Registry.Gather(); err == nil {
		t.Fatal("galat query seharusnya dilaporkan Gather")
	}
	// sampel gauge dilewati: lebih baik tidak ada data daripada angka 0 yang menyesatkan
	if g := find(t, m.Registry, "mail_outbox_pending", nil); g != nil {
		t.Fatalf("saat query gagal sampel harus dilewati, dapat %v", g)
	}
}
