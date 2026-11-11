package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"qms/mail-service/internal/apperror"
	"qms/mail-service/internal/mailtemplate"
	"qms/mail-service/internal/model"
	"qms/mail-service/internal/service"
)

type row struct {
	model.OutboxItem
	sent, failed bool
	lastErr      string
	retryIn      time.Duration
}

type memStore struct {
	mu   sync.Mutex
	rows map[int64]*row
}

func newStore(items ...model.OutboxItem) *memStore {
	s := &memStore{rows: map[int64]*row{}}
	for _, it := range items {
		it.Attempts = 0
		s.rows[it.ID] = &row{OutboxItem: it}
	}
	return s
}

func (s *memStore) Claim(_ context.Context, limit int, _ time.Duration) ([]model.OutboxItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []model.OutboxItem
	for id := int64(1); id <= int64(len(s.rows)) && len(out) < limit; id++ {
		r := s.rows[id]
		if r.sent || r.failed || r.retryIn > 0 {
			continue
		}
		r.Attempts++
		out = append(out, r.OutboxItem)
	}
	return out, nil
}
func (s *memStore) MarkSent(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].sent = true
	return nil
}
func (s *memStore) MarkRetry(_ context.Context, id int64, cause string, d time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].lastErr, s.rows[id].retryIn = cause, d
	return nil
}
func (s *memStore) MarkFailed(_ context.Context, id int64, cause string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows[id].failed, s.rows[id].lastErr = true, cause
	return nil
}
func (s *memStore) get(id int64) row {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.rows[id]
}

type sender struct {
	mu   sync.Mutex
	got  []model.Message
	errs []error // diambil berurutan; habis = sukses
}

func (s *sender) Send(_ context.Context, m model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.errs) > 0 {
		e := s.errs[0]
		s.errs = s.errs[1:]
		if e != nil {
			return e
		}
	}
	s.got = append(s.got, m)
	return nil
}
func (s *sender) sent() []model.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Message(nil), s.got...)
}

func item(id int64, kind, email, payload string) model.OutboxItem {
	return model.OutboxItem{ID: id, Kind: kind, Email: email, Payload: json.RawMessage(payload)}
}

func pl(name, queue string, number int) string {
	b, _ := json.Marshal(map[string]any{"queue_id": 7, "queue_name": queue, "prefix": "B", "number": number, "name": name})
	return string(b)
}

func newWorker(st service.OutboxStore, sd service.Sender) *service.OutboxService {
	return service.NewOutboxService(st, sd, mailtemplate.New(), service.OutboxConfig{
		PublicOrigin: "https://qms.example/", Rate: 1000, MaxAttempts: 3, Poll: 10 * time.Millisecond,
		BackoffBase: time.Minute, BackoffMax: 4 * time.Minute,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// runUntil menjalankan worker sampai cond benar atau waktu habis.
func runUntil(t *testing.T, w *service.OutboxService, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if !cond() {
		t.Fatal("kondisi tidak tercapai sebelum batas waktu")
	}
}

func TestLabel_MatchesTicketFormat(t *testing.T) {
	for _, c := range []struct {
		p    string
		n    int
		want string
	}{{"A", 1, "A-001"}, {"", 12, "A-012"}, {"B2", 1234, "B2-1234"}} {
		if got := service.Label(c.p, c.n); got != c.want {
			t.Errorf("Label(%q,%d)=%q, want %q", c.p, c.n, got, c.want)
		}
	}
}

func TestWorker_SendsFourKindsInIndonesianWithTicketLink(t *testing.T) {
	st := newStore(
		item(1, "joined", "a@x.test", pl("Budi", "Loket Pajak", 5)),
		item(2, "cancelled", "a@x.test", pl("Budi", "Loket Pajak", 5)),
		item(3, "removed", "a@x.test", pl("Budi", "Loket Pajak", 5)),
		item(4, "completed", "a@x.test", pl("Budi", "Loket Pajak", 5)),
	)
	sd := &sender{}
	runUntil(t, newWorker(st, sd), func() bool { return len(sd.sent()) == 4 })
	want := []string{"Anda masuk antrian Loket Pajak: nomor B-005", "Antrian Loket Pajak dibatalkan: nomor B-005",
		"Nomor B-005 di Loket Pajak dihapus oleh penyelenggara", "Selesai dilayani: nomor B-005 di Loket Pajak"}
	for i, m := range sd.sent() {
		if m.Subject != want[i] {
			t.Errorf("subject %d = %q, want %q", i, m.Subject, want[i])
		}
		if m.To != "a@x.test" {
			t.Errorf("To = %q", m.To)
		}
		for _, body := range []string{m.Text, m.HTML} {
			if !strings.Contains(body, "https://qms.example/ticket/7") {
				t.Errorf("email %d tanpa tautan /ticket/7: %q", i, body)
			}
			if !strings.Contains(body, "B-005") || !strings.Contains(body, "Budi") {
				t.Errorf("email %d tanpa nomor atau nama", i)
			}
		}
	}
	for id := int64(1); id <= 4; id++ {
		if !st.get(id).sent {
			t.Errorf("baris %d belum ditandai terkirim", id)
		}
	}
}

func TestWorker_EscapesUserControlledValuesInHTMLOnly(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", pl(`<img src=x onerror=1>`, `Q <b>"&"</b>`, 1)))
	sd := &sender{}
	runUntil(t, newWorker(st, sd), func() bool { return len(sd.sent()) == 1 })
	m := sd.sent()[0]
	if strings.Contains(m.HTML, "<img") || strings.Contains(m.HTML, "<b>\"") {
		t.Errorf("HTML tidak di-escape: %s", m.HTML)
	}
	if !strings.Contains(m.Text, "<img src=x onerror=1>") {
		t.Errorf("teks biasa seharusnya apa adanya: %s", m.Text)
	}
}

func TestWorker_NeverIncludesOwnerNote(t *testing.T) {
	// payload dari trigger tidak memuat catatan; walau ada kunci tak dikenal, tidak boleh bocor ke email.
	st := newStore(item(1, "completed", "a@x.test",
		`{"queue_id":7,"queue_name":"Q","prefix":"A","number":1,"name":"N","note":"RAHASIA OWNER"}`))
	sd := &sender{}
	runUntil(t, newWorker(st, sd), func() bool { return len(sd.sent()) == 1 })
	m := sd.sent()[0]
	if strings.Contains(m.Text+m.HTML+m.Subject, "RAHASIA") {
		t.Error("catatan owner bocor ke email")
	}
}

func TestWorker_TransientFailureSchedulesBackoffThenSucceeds(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)))
	sd := &sender{errs: []error{errors.New("connection refused")}}
	runUntil(t, newWorker(st, sd), func() bool { return st.get(1).retryIn > 0 })
	r := st.get(1)
	if r.sent || r.failed || r.retryIn != time.Minute || !strings.Contains(r.lastErr, "refused") {
		t.Fatalf("setelah gagal sementara: %+v", r)
	}
}

func TestWorker_BackoffDoublesAndIsCapped(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)))
	sd := &sender{errs: []error{errors.New("e1"), errors.New("e2")}}
	w := newWorker(st, sd)
	for i, want := range []time.Duration{time.Minute, 2 * time.Minute} {
		runUntil(t, w, func() bool { return st.get(1).retryIn > 0 && st.get(1).Attempts == i+1 })
		if got := st.get(1).retryIn; got != want {
			t.Fatalf("percobaan %d: jeda %v, want %v", i+1, got, want)
		}
		st.mu.Lock()
		st.rows[1].retryIn = 0 // anggap sudah jatuh tempo
		st.mu.Unlock()
	}
}

func TestWorker_PermanentRejectionIsNotRetried(t *testing.T) {
	st := newStore(item(1, "joined", "bad@x.test", pl("N", "Q", 1)))
	sd := &sender{errs: []error{fmt.Errorf("%w: 550 no such user", apperror.ErrPermanent)}}
	runUntil(t, newWorker(st, sd), func() bool { return st.get(1).failed })
	if r := st.get(1); r.Attempts != 1 || !strings.Contains(r.lastErr, "550") {
		t.Fatalf("%+v", r)
	}
}

func TestWorker_GivesUpAfterMaxAttempts(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", pl("N", "Q", 1)))
	sd := &sender{errs: []error{errors.New("x"), errors.New("x"), errors.New("x")}}
	w := newWorker(st, sd) // MaxAttempts = 3
	for i := 0; i < 3 && !st.get(1).failed; i++ {
		runUntil(t, w, func() bool { return st.get(1).retryIn > 0 || st.get(1).failed })
		st.mu.Lock()
		st.rows[1].retryIn = 0
		st.mu.Unlock()
	}
	if r := st.get(1); !r.failed || r.Attempts != 3 || !strings.Contains(r.lastErr, "gave up") {
		t.Fatalf("%+v", r)
	}
}

func TestWorker_BrokenPayloadOrUnknownKindFailsWithoutSending(t *testing.T) {
	st := newStore(
		item(1, "joined", "a@x.test", `{"queue_id":0}`),
		item(2, "teleport", "a@x.test", pl("N", "Q", 1)),
		item(3, "joined", "", pl("N", "Q", 1)),
		item(4, "joined", "a@x.test", `not json`),
	)
	sd := &sender{}
	runUntil(t, newWorker(st, sd), func() bool {
		return st.get(1).failed && st.get(2).failed && st.get(3).failed && st.get(4).failed
	})
	if len(sd.sent()) != 0 {
		t.Fatal("tidak boleh ada email terkirim")
	}
}

func TestWorker_OneBadRowDoesNotBlockOthers(t *testing.T) {
	st := newStore(item(1, "joined", "a@x.test", `not json`), item(2, "joined", "b@x.test", pl("N", "Q", 2)))
	sd := &sender{}
	runUntil(t, newWorker(st, sd), func() bool { return len(sd.sent()) == 1 })
	if sd.sent()[0].To != "b@x.test" {
		t.Fatal("email kedua tidak terkirim")
	}
}

func TestWorker_RateLimitSpacesSends(t *testing.T) {
	var items []model.OutboxItem
	for i := int64(1); i <= 6; i++ {
		items = append(items, item(i, "joined", "a@x.test", pl("N", "Q", int(i))))
	}
	st := newStore(items...)
	sd := &sender{}
	w := service.NewOutboxService(st, sd, mailtemplate.New(), service.OutboxConfig{PublicOrigin: "http://h", Rate: 20, Poll: 10 * time.Millisecond},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	start := time.Now()
	runUntil(t, w, func() bool { return len(sd.sent()) == 6 })
	// 6 email pada 20/detik: minimal sekitar 5 interval (250 ms); toleransi untuk ticker pertama.
	if el := time.Since(start); el < 200*time.Millisecond {
		t.Fatalf("terkirim terlalu cepat (%v): rate limit tidak bekerja", el)
	}
}

func TestWorker_StopsPromptlyOnCancel(t *testing.T) {
	st := newStore()
	w := newWorker(st, &sender{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker tidak berhenti")
	}
}

func TestWorker_WakeIsNonBlocking(t *testing.T) {
	w := newWorker(newStore(), &sender{})
	for i := 0; i < 100; i++ {
		w.Wake() // tanpa penerima; tidak boleh menggantung
	}
}

func TestWorker_QueueClosedGoesToOwnerWithHistoryLink(t *testing.T) {
	st := newStore(item(1, "queue-closed", "owner@x.test", pl("Sari", "Loket Pajak", 0)))
	sd := &sender{}
	runUntil(t, newWorker(st, sd), func() bool { return len(sd.sent()) == 1 })
	m := sd.sent()[0]
	if m.To != "owner@x.test" {
		t.Errorf("To = %q", m.To)
	}
	if m.Subject != "Antrian Loket Pajak sudah ditutup" {
		t.Errorf("subjek = %q", m.Subject)
	}
	for _, body := range []string{m.Text, m.HTML} {
		if !strings.Contains(body, "https://qms.example/riwayat") || strings.Contains(body, "/ticket/") {
			t.Errorf("tautan salah: %q", body)
		}
		if !strings.Contains(body, "Sari") || !strings.Contains(body, "Loket Pajak") {
			t.Errorf("tanpa nama atau antrian: %q", body)
		}
	}
	if !st.get(1).sent {
		t.Error("baris belum ditandai terkirim")
	}
}
