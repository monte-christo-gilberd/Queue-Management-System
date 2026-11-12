package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBackend meniru bagian API yang dipakai penahan: QR, join, SSE, status, dan Next (yang menyiarkan event ke
// semua koneksi SSE antrian itu). dropAfter > 0 memutus tiap koneksi SSE setelah selang itu (meniru koneksi yang hilang).
type fakeBackend struct {
	mu        sync.Mutex
	subs      map[string][]chan string // queue id -> saluran event tiap koneksi
	dropAfter time.Duration
	statusN   int
}

// newFakeBackend membuat server uji dan mengembalikan alamatnya.
func newFakeBackend(t *testing.T, dropAfter time.Duration) (*fakeBackend, *httptest.Server) {
	t.Helper()
	fb := &fakeBackend{subs: map[string][]chan string{}, dropAfter: dropAfter}
	mux := http.NewServeMux()
	// QR: join_url dengan token palsu
	mux.HandleFunc("GET /api/v1/queues/{id}/qr", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"data":{"join_url":"http://x/join?t=tok-%s"}}`, r.PathValue("id"))
	})
	// join: selalu sukses
	mux.HandleFunc("POST /api/v1/join", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"data":{"id":1}}`) })
	// status: dihitung agar tes bisa memastikan peniruan klien memanggilnya
	mux.HandleFunc("GET /api/v1/queues/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		fb.mu.Lock()
		fb.statusN++
		fb.mu.Unlock()
		fmt.Fprint(w, `{"data":{}}`)
	})
	// Next: siarkan satu event ticket_next (dengan ts sekarang) ke semua koneksi antrian itu
	mux.HandleFunc("POST /api/v1/queues/{id}/next", func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf(`{"type":"ticket_next","queue_id":1,"ts":%q}`, time.Now().UTC().Format(time.RFC3339Nano))
		fb.mu.Lock()
		for _, ch := range fb.subs[r.PathValue("id")] {
			select {
			case ch <- msg:
			default:
			}
		}
		fb.mu.Unlock()
		fmt.Fprint(w, `{"data":{}}`)
	})
	// SSE: kirim heartbeat awal, lalu event yang disiarkan, sampai klien putus (atau dropAfter lewat)
	mux.HandleFunc("GET /api/v1/queues/{id}/event", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl := w.(http.Flusher)
		ch := make(chan string, 8)
		id := r.PathValue("id")
		fb.mu.Lock()
		fb.subs[id] = append(fb.subs[id], ch)
		fb.mu.Unlock()
		fmt.Fprint(w, ": ping\n\n")
		fl.Flush()
		var drop <-chan time.Time
		if fb.dropAfter > 0 {
			drop = time.After(fb.dropAfter)
		}
		for {
			select {
			case m := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", m)
				fl.Flush()
			case <-drop:
				return
			case <-r.Context().Done():
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return fb, srv
}

// testTokens membuat 2 antrian (owner 1 dan 2) dan n pelanggan.
func testTokens(n int) *Tokens {
	tk := &Tokens{
		Owners: []Account{{ID: 1, Token: "o1"}, {ID: 2, Token: "o2"}},
		Queues: []Queue{{ID: 10, OwnerID: 1}, {ID: 11, OwnerID: 2}},
	}
	for i := 0; i < n; i++ {
		tk.Customers = append(tk.Customers, Account{ID: int64(100 + i), Token: fmt.Sprintf("c%d", i)})
	}
	return tk
}

// Alur penuh: semua koneksi terbuka dan bertahan, Next memicu event, event diterima dengan latensi terukur, dan peniruan
// klien memanggil status setelah event.
func TestHolder_HoldsConnectionsReceivesEventsAndMimicsClient(t *testing.T) {
	fb, srv := newFakeBackend(t, 0)
	cfg := Config{BaseURL: srv.URL, Conns: 6, Ramp: 200, Duration: 1500 * time.Millisecond, Mimic: true,
		Debounce: 50 * time.Millisecond, NextInterval: 300 * time.Millisecond, PreloadWorkers: 4}
	h := newHolder(cfg, testTokens(6))

	rep, err := h.run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// semua 6 koneksi terbuka dan masih terbuka saat uji selesai: 100% bertahan
	if rep.PeakConnected != 6 || rep.FinalConnected != 6 || rep.HeldPercent != 100 {
		t.Fatalf("koneksi tidak bertahan: puncak %d, akhir %d, %.1f%%", rep.PeakConnected, rep.FinalConnected, rep.HeldPercent)
	}
	if rep.ConnectErrors != 0 || rep.Dropped != 0 {
		t.Fatalf("tidak boleh ada galat: %+v", rep)
	}
	// Next dipicu dan event diterima (3 pelanggan per antrian, lebih dari satu Next)
	if rep.NextCalls == 0 || rep.Events == 0 || rep.Heartbeats == 0 {
		t.Fatalf("event tidak mengalir: next %d, event %d, heartbeat %d", rep.NextCalls, rep.Events, rep.Heartbeats)
	}
	// latensi event terukur dan masuk akal (server uji lokal: jauh di bawah satu detik)
	if rep.EventLatency.Count == 0 || rep.EventLatency.P99 <= 0 || rep.EventLatency.P99 > 1000 {
		t.Fatalf("latensi event tidak masuk akal: %+v", rep.EventLatency)
	}
	// peniruan klien: status dipanggil setelah event, tanpa galat
	fb.mu.Lock()
	served := fb.statusN
	fb.mu.Unlock()
	if rep.StatusCalls == 0 || served == 0 || rep.StatusErrors != 0 {
		t.Fatalf("peniruan klien tidak memanggil status: calls %d, served %d, errors %d", rep.StatusCalls, served, rep.StatusErrors)
	}
}

// Tanpa -mimic, event tidak memicu GET /status.
func TestHolder_NoMimicNoStatusCalls(t *testing.T) {
	_, srv := newFakeBackend(t, 0)
	cfg := Config{BaseURL: srv.URL, Conns: 4, Ramp: 200, Duration: time.Second, Mimic: false,
		Debounce: 20 * time.Millisecond, NextInterval: 250 * time.Millisecond, PreloadWorkers: 2}

	rep, err := newHolder(cfg, testTokens(4)).run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if rep.Events == 0 || rep.StatusCalls != 0 {
		t.Fatalf("mimic mati: event %d (harus > 0), status %d (harus 0)", rep.Events, rep.StatusCalls)
	}
}

// Koneksi yang diputus server di tengah uji dihitung sebagai putus, dan persentase bertahan turun di bawah gerbang.
func TestHolder_CountsDroppedConnections(t *testing.T) {
	_, srv := newFakeBackend(t, 400*time.Millisecond)
	cfg := Config{BaseURL: srv.URL, Conns: 4, Ramp: 200, Duration: 1200 * time.Millisecond, Mimic: false, PreloadWorkers: 2}

	rep, err := newHolder(cfg, testTokens(4)).run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if rep.Dropped != 4 || rep.FinalConnected != 0 || rep.HeldPercent >= 99.5 {
		t.Fatalf("pemutusan tidak terdeteksi: dropped %d, akhir %d, %.1f%%", rep.Dropped, rep.FinalConnected, rep.HeldPercent)
	}
}

// Meminta lebih banyak koneksi daripada pelanggan di seed harus ditolak dengan pesan yang jelas.
func TestHolder_RejectsMoreConnsThanCustomers(t *testing.T) {
	_, srv := newFakeBackend(t, 0)
	cfg := Config{BaseURL: srv.URL, Conns: 10, Ramp: 100, Duration: time.Second, PreloadWorkers: 2}

	_, err := newHolder(cfg, testTokens(3)).run(context.Background())

	if err == nil || !strings.Contains(err.Error(), "melebihi jumlah pelanggan") {
		t.Fatalf("seharusnya ditolak: %v", err)
	}
}

// Pagar target: host produksi ditolak mutlak, host non-privat butuh izin eksplisit, host privat lolos.
func TestAssertSafeTarget(t *testing.T) {
	cases := []struct {
		base        string
		allowPublic bool
		extra       string
		wantErr     bool
	}{
		{"http://localhost:18080", false, "", false},
		{"http://127.0.0.1:8080", false, "", false},
		{"http://10.1.2.3", false, "", false},
		{"http://172.31.0.5:80", false, "", false},
		{"http://192.168.1.9", false, "", false},
		{"http://172.32.0.1", false, "", true}, // di luar rentang privat 172.16-31
		{"https://qms.montechristo.my.id", false, "", true},
		{"https://qms.montechristo.my.id", true, "", true}, // produksi ditolak walau allowPublic
		{"http://203.0.113.5", false, "", true},
		{"http://203.0.113.5", true, "", false},           // mesin uji publik dengan izin
		{"http://203.0.113.5", true, "203.0.113.5", true}, // daftar blokir tambahan menang
		{"bukan url", false, "", true},
	}
	for _, c := range cases {
		err := assertSafeTarget(c.base, c.allowPublic, c.extra)
		if (err != nil) != c.wantErr {
			t.Errorf("assertSafeTarget(%q, allow=%v, extra=%q) err=%v, wantErr=%v", c.base, c.allowPublic, c.extra, err, c.wantErr)
		}
	}
}

// Persentil memakai metode peringkat terdekat dan menangani masukan kosong.
func TestSummarizePercentiles(t *testing.T) {
	if got := summarize(nil); got.Count != 0 || got.P99 != 0 {
		t.Fatalf("kosong seharusnya nol: %+v", got)
	}
	var s []time.Duration
	for i := 1; i <= 100; i++ {
		s = append(s, time.Duration(i)*time.Millisecond)
	}
	got := summarize(s)
	if got.Count != 100 || got.P50 != 50 || got.P95 != 95 || got.P99 != 99 || got.Max != 100 {
		t.Fatalf("persentil salah: %+v", got)
	}
}
