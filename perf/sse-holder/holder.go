package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config berisi semua pengaturan penahan koneksi.
type Config struct {
	// BaseURL: alamat nginx uji tanpa garis miring di akhir.
	BaseURL string
	// Conns: jumlah koneksi SSE yang ditahan.
	Conns int
	// Ramp: berapa koneksi baru dibuka per detik (menghindari lonjakan SYN yang tidak realistis).
	Ramp int
	// Duration: lama menahan koneksi setelah semuanya terbuka.
	Duration time.Duration
	// Mimic: tiru klien nyata, yaitu setelah event (kecuali qr_updated) panggil GET /status setelah debounce.
	Mimic bool
	// Debounce: jeda penggabungan event sebelum memanggil status (sesuai frontend use-queue-spot.ts: 300 ms).
	Debounce time.Duration
	// NextInterval: jeda antar Next per antrian oleh owner untuk memicu event. 0 = tidak memicu.
	NextInterval time.Duration
	// PreloadWorkers: goroutine yang membuat tiket pelanggan sebelum SSE dibuka.
	PreloadWorkers int
}

// holder menjalankan satu uji penahan koneksi.
type holder struct {
	cfg    Config
	data   *Tokens
	stats  *Stats
	api    *http.Client // klien untuk join, status, next (keep-alive, dibatasi timeout)
	stream *http.Client // klien untuk SSE (tanpa timeout total; satu request per koneksi)
}

// newHolder menyiapkan klien HTTP. Transport SSE memakai koneksi sendiri per request dan tanpa HTTP/2 (satu stream per
// koneksi TCP, seperti browser lewat nginx HTTP/1.1).
func newHolder(cfg Config, data *Tokens) *holder {
	api := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns: 2000, MaxIdleConnsPerHost: 2000, IdleConnTimeout: 60 * time.Second,
			ForceAttemptHTTP2: false,
		},
	}
	stream := &http.Client{
		// tanpa Timeout: koneksi SSE berumur panjang; pembatalan lewat context
		Transport: &http.Transport{
			DisableKeepAlives: true, MaxIdleConns: 0, ForceAttemptHTTP2: false,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	return &holder{cfg: cfg, data: data, stats: &Stats{}, api: api, stream: stream}
}

// do mengirim request ber-token dan mengembalikan status serta body (maks 1 MiB dibaca).
func (h *holder) do(ctx context.Context, method, path, token string, body io.Reader) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, h.cfg.BaseURL+path, body)
	if err != nil {
		return 0, nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.api.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, b, nil
}

// joinTokens mengambil token join tiap antrian (owner membaca QR statis). Hasil: queueID -> token join.
func (h *holder) joinTokens(ctx context.Context) (map[int64]string, error) {
	out := make(map[int64]string, len(h.data.Queues))
	for _, q := range h.data.Queues {
		owner := h.data.owner(q.OwnerID)
		if owner == nil {
			return nil, fmt.Errorf("owner %d dari antrian %d tidak ada di tokens.json", q.OwnerID, q.ID)
		}
		code, body, err := h.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/queues/%d/qr", q.ID), owner.Token, nil)
		if err != nil || code != http.StatusOK {
			return nil, fmt.Errorf("ambil QR antrian %d: status %d err %v", q.ID, code, err)
		}
		var resp struct {
			Data struct {
				JoinURL string `json:"join_url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		i := strings.Index(resp.Data.JoinURL, "t=")
		if i < 0 {
			return nil, fmt.Errorf("join_url antrian %d tanpa token", q.ID)
		}
		out[q.ID] = resp.Data.JoinURL[i+2:]
	}
	return out, nil
}

// preload membuat tiket untuk semua pelanggan yang akan menahan SSE (hanya pemegang tiket dan owner yang boleh subscribe).
// Mengembalikan jumlah gagal. Join bersifat idempoten: pelanggan yang sudah punya tiket mendapat tiketnya kembali.
func (h *holder) preload(ctx context.Context, tokens map[int64]string) int {
	var wg sync.WaitGroup
	jobs := make(chan int)
	var mu sync.Mutex
	failed := 0
	for w := 0; w < h.cfg.PreloadWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				q := h.queueOf(i)
				payload, _ := json.Marshal(map[string]string{"t": tokens[q.ID]})
				code, _, err := h.do(ctx, http.MethodPost, "/api/v1/join", h.data.Customers[i].Token, strings.NewReader(string(payload)))
				if err != nil || code != http.StatusOK {
					mu.Lock()
					failed++
					mu.Unlock()
				}
			}
		}()
	}
	for i := 0; i < h.cfg.Conns; i++ {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return failed
}

// queueOf membagi pelanggan ke antrian secara berputar (sama dengan skrip k6 lib/data.js).
func (h *holder) queueOf(customerIndex int) Queue {
	return h.data.Queues[customerIndex%len(h.data.Queues)]
}

// run menjalankan seluruh uji: preload, ramp koneksi, pemicu Next, tahan, lalu laporan.
func (h *holder) run(ctx context.Context) (Report, error) {
	if h.cfg.Conns > len(h.data.Customers) {
		return Report{}, fmt.Errorf("conns %d melebihi jumlah pelanggan di seed (%d): jalankan make perf-seed PERF_CUSTOMERS=...", h.cfg.Conns, len(h.data.Customers))
	}
	tokens, err := h.joinTokens(ctx)
	if err != nil {
		return Report{}, err
	}
	if failed := h.preload(ctx, tokens); failed > 0 {
		return Report{}, fmt.Errorf("preload: %d dari %d pelanggan gagal bergabung", failed, h.cfg.Conns)
	}

	start := time.Now()
	// konteks koneksi: dibatalkan saat durasi habis atau penahan dihentikan
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup

	// buka koneksi dengan laju Ramp per detik
	interval := time.Second / time.Duration(maxInt(h.cfg.Ramp, 1))
	tick := time.NewTicker(interval)
	for i := 0; i < h.cfg.Conns; i++ {
		select {
		case <-tick.C:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h.watch(connCtx, i)
		}(i)
	}
	tick.Stop()

	// pemicu Next: owner tiap antrian menekan Next berkala sehingga event mengalir
	if h.cfg.NextInterval > 0 {
		for _, q := range h.data.Queues {
			wg.Add(1)
			go func(q Queue) {
				defer wg.Done()
				h.triggerNext(connCtx, q)
			}(q)
		}
	}

	// tahan selama Duration (dihitung setelah semua koneksi diminta), lalu berhenti
	select {
	case <-time.After(h.cfg.Duration):
	case <-ctx.Done():
	}
	// foto keadaan SEBELUM menutup: final_connected harus mencerminkan koneksi yang bertahan sampai akhir uji
	rep := h.stats.report(h.cfg.Conns, time.Since(start))
	cancel()
	wg.Wait()
	return rep, nil
}

// triggerNext menekan Next untuk satu antrian tiap NextInterval sampai konteks batal.
func (h *holder) triggerNext(ctx context.Context, q Queue) {
	owner := h.data.owner(q.OwnerID)
	if owner == nil {
		return
	}
	t := time.NewTicker(h.cfg.NextInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			code, _, err := h.do(ctx, http.MethodPost, fmt.Sprintf("/api/v1/queues/%d/next", q.ID), owner.Token, strings.NewReader("{}"))
			h.stats.NextCalls.Add(1)
			// 409 (QUEUE_EMPTY, COUNTER_STATE_CHANGED) adalah jawaban sah, bukan galat
			if err != nil || (code != http.StatusOK && code != http.StatusConflict) {
				h.stats.NextErrors.Add(1)
			}
		case <-ctx.Done():
			return
		}
	}
}

// watch membuka satu koneksi SSE untuk pelanggan ke-i dan membaca stream sampai konteks batal atau koneksi putus.
func (h *holder) watch(ctx context.Context, i int) {
	q := h.queueOf(i)
	cust := h.data.Customers[i]
	h.stats.ConnectAttempts.Add(1)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/queues/%d/event", h.cfg.BaseURL, q.ID), nil)
	if err != nil {
		h.stats.ConnectErrors.Add(1)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cust.Token)
	req.Header.Set("Accept", "text/event-stream")
	t0 := time.Now()
	resp, err := h.stream.Do(req)
	if err != nil {
		if ctx.Err() == nil {
			h.stats.ConnectErrors.Add(1)
		}
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		h.stats.ConnectErrors.Add(1)
		return
	}
	h.stats.addConnect(time.Since(t0))
	h.stats.connUp()
	defer h.stats.connDown()

	var debounce *time.Timer
	if h.cfg.Mimic {
		defer func() {
			if debounce != nil {
				debounce.Stop()
			}
		}()
	}
	// satu goroutine pembaca per koneksi; konteks dibatalkan menutup body dan membuat ReadString kembali dengan galat
	reader := bufio.NewReaderSize(resp.Body, 4096)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// putus tanpa pembatalan = koneksi hilang di tengah uji
			if ctx.Err() == nil {
				h.stats.Dropped.Add(1)
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, ": ping"):
			h.stats.Heartbeats.Add(1)
		case strings.HasPrefix(line, "data: "):
			h.onEvent(ctx, line[len("data: "):], q.ID, cust.Token, &debounce)
		}
	}
}

// eventMsg adalah bentuk event SSE dari backend (internal/sse/message.go).
type eventMsg struct {
	Type string    `json:"type"`
	TS   time.Time `json:"ts"`
}

// onEvent mencatat latensi event dan, bila Mimic, menjadwalkan GET /status setelah debounce (digabung bila event beruntun).
func (h *holder) onEvent(ctx context.Context, raw string, queueID int64, token string, debounce **time.Timer) {
	var ev eventMsg
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return
	}
	h.stats.Events.Add(1)
	if !ev.TS.IsZero() {
		h.stats.addEvent(time.Since(ev.TS))
	}
	// frontend tidak memuat ulang status untuk perubahan QR (use-queue-spot.ts)
	if !h.cfg.Mimic || ev.Type == "queue_qr_updated" {
		return
	}
	if *debounce != nil {
		(*debounce).Stop()
	}
	*debounce = time.AfterFunc(h.cfg.Debounce, func() {
		if ctx.Err() != nil {
			return
		}
		t0 := time.Now()
		code, _, err := h.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/queues/%d/status", queueID), token, nil)
		h.stats.StatusCalls.Add(1)
		if err != nil || code != http.StatusOK {
			h.stats.StatusErrors.Add(1)
			return
		}
		h.stats.addStatus(time.Since(t0))
	})
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
