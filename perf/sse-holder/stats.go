package main

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Stats mengumpulkan angka uji dari ribuan goroutine koneksi. Penghitung memakai atomic; sampel latensi dikumpulkan di
// slice per kategori yang dilindungi mutex (penulisan hanya terjadi per event, jauh lebih jarang dari baca byte).
type Stats struct {
	// ConnectAttempts: berapa kali membuka koneksi SSE dicoba.
	ConnectAttempts atomic.Int64
	// ConnectErrors: percobaan yang gagal sebelum stream terbuka (galat jaringan atau status bukan 200).
	ConnectErrors atomic.Int64
	// Connected: koneksi yang sedang terbuka sekarang.
	Connected atomic.Int64
	// PeakConnected: nilai Connected tertinggi selama uji.
	PeakConnected atomic.Int64
	// Dropped: koneksi yang sudah terbuka lalu putus sebelum uji selesai (bukan karena penahan berhenti).
	Dropped atomic.Int64
	// Events: jumlah event data yang diterima (tidak termasuk heartbeat).
	Events atomic.Int64
	// Heartbeats: jumlah komentar ": ping" yang diterima.
	Heartbeats atomic.Int64
	// StatusCalls dan StatusErrors: panggilan GET /status hasil peniruan klien, dan yang gagal (bukan 200).
	StatusCalls  atomic.Int64
	StatusErrors atomic.Int64
	// NextCalls dan NextErrors: panggilan Next oleh owner untuk memicu event. 409 QUEUE_EMPTY bukan galat.
	NextCalls  atomic.Int64
	NextErrors atomic.Int64

	mu sync.Mutex
	// eventLatency: selang waktu event dibuat backend (ts) sampai diterima penahan.
	eventLatency []time.Duration
	// statusLatency: lama GET /status hasil peniruan klien.
	statusLatency []time.Duration
	// connectLatency: lama membuka koneksi SSE sampai header respons diterima.
	connectLatency []time.Duration
}

// addEvent mencatat satu latensi event.
func (s *Stats) addEvent(d time.Duration) {
	s.mu.Lock()
	s.eventLatency = append(s.eventLatency, d)
	s.mu.Unlock()
}

// addStatus mencatat satu latensi GET /status.
func (s *Stats) addStatus(d time.Duration) {
	s.mu.Lock()
	s.statusLatency = append(s.statusLatency, d)
	s.mu.Unlock()
}

// addConnect mencatat satu latensi pembukaan koneksi.
func (s *Stats) addConnect(d time.Duration) {
	s.mu.Lock()
	s.connectLatency = append(s.connectLatency, d)
	s.mu.Unlock()
}

// connUp menandai satu koneksi terbuka dan memperbarui puncak.
func (s *Stats) connUp() {
	n := s.Connected.Add(1)
	for {
		peak := s.PeakConnected.Load()
		if n <= peak || s.PeakConnected.CompareAndSwap(peak, n) {
			return
		}
	}
}

// connDown menandai satu koneksi tertutup.
func (s *Stats) connDown() { s.Connected.Add(-1) }

// Percentiles meringkas satu kumpulan latensi dalam milidetik.
type Percentiles struct {
	Count int     `json:"count"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
	Max   float64 `json:"max_ms"`
}

// summarize menghitung persentil (metode peringkat terdekat) dari sampel; kosong menghasilkan semua nol.
func summarize(samples []time.Duration) Percentiles {
	if len(samples) == 0 {
		return Percentiles{}
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	at := func(p float64) float64 {
		idx := int(float64(len(sorted))*p+0.5) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		return float64(sorted[idx]) / float64(time.Millisecond)
	}
	return Percentiles{
		Count: len(sorted), P50: at(0.50), P95: at(0.95), P99: at(0.99),
		Max: float64(sorted[len(sorted)-1]) / float64(time.Millisecond),
	}
}

// Report adalah ringkasan akhir uji (dicetak dan ditulis sebagai JSON).
type Report struct {
	Target          int         `json:"target_connections"`
	PeakConnected   int64       `json:"peak_connected"`
	FinalConnected  int64       `json:"final_connected"`
	HeldPercent     float64     `json:"held_percent"`
	ConnectAttempts int64       `json:"connect_attempts"`
	ConnectErrors   int64       `json:"connect_errors"`
	Dropped         int64       `json:"dropped_after_open"`
	Events          int64       `json:"events"`
	Heartbeats      int64       `json:"heartbeats"`
	StatusCalls     int64       `json:"status_calls"`
	StatusErrors    int64       `json:"status_errors"`
	NextCalls       int64       `json:"next_calls"`
	NextErrors      int64       `json:"next_errors"`
	EventLatency    Percentiles `json:"event_latency"`
	StatusLatency   Percentiles `json:"status_latency"`
	ConnectLatency  Percentiles `json:"connect_latency"`
	DurationSeconds float64     `json:"duration_seconds"`
}

// report menyusun Report dari keadaan sekarang. target = jumlah koneksi yang diminta.
func (s *Stats) report(target int, duration time.Duration) Report {
	s.mu.Lock()
	ev := append([]time.Duration(nil), s.eventLatency...)
	st := append([]time.Duration(nil), s.statusLatency...)
	cn := append([]time.Duration(nil), s.connectLatency...)
	s.mu.Unlock()
	final := s.Connected.Load()
	held := 0.0
	if target > 0 {
		held = float64(final) / float64(target) * 100
	}
	return Report{
		Target: target, PeakConnected: s.PeakConnected.Load(), FinalConnected: final, HeldPercent: held,
		ConnectAttempts: s.ConnectAttempts.Load(), ConnectErrors: s.ConnectErrors.Load(), Dropped: s.Dropped.Load(),
		Events: s.Events.Load(), Heartbeats: s.Heartbeats.Load(),
		StatusCalls: s.StatusCalls.Load(), StatusErrors: s.StatusErrors.Load(),
		NextCalls: s.NextCalls.Load(), NextErrors: s.NextErrors.Load(),
		EventLatency: summarize(ev), StatusLatency: summarize(st), ConnectLatency: summarize(cn),
		DurationSeconds: duration.Seconds(),
	}
}
