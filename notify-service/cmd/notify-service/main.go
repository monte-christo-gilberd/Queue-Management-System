// Command notify-service menjalankan layanan notifikasi push QMS: mendengarkan event dari database
// (LISTEN queue_events), menentukan penerima, lalu mengirim Web Push ke browser.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"qms/notify-service/internal/config"
	"qms/notify-service/internal/event"
	"qms/notify-service/internal/notify"
	"qms/notify-service/internal/push"
	"qms/notify-service/internal/store"

	"github.com/jackc/pgx/v5/pgxpool"
)

// fatal mencatat galat dengan pesan msg lalu menghentikan proses dengan kode 1.
func fatal(msg string, err error) {
	// Catat galat dalam format JSON sebelum proses berhenti.
	slog.Error(msg, "err", err)
	// Hentikan proses dengan status gagal.
	os.Exit(1)
}

// main menyusun konfigurasi, koneksi, health server, dan listener, lalu menunggu sinyal berhenti.
func main() {
	// Atur logger default ke JSON di stdout untuk seluruh proses.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Baca dan validasi konfigurasi. Galat berarti variabel wajib belum diisi.
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}

	// Buat konteks yang batal saat SIGINT atau SIGTERM diterima.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Buat pool koneksi ke PostgreSQL. Galat berarti URL salah atau database tidak bisa dijangkau.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("db pool", err)
	}
	// Tutup pool saat main selesai.
	defer pool.Close()

	// /health: 200 hanya bila LISTEN queue_events sedang tersambung.
	var listening atomic.Bool
	// Buat router HTTP untuk health check.
	mux := http.NewServeMux()
	// Handler health: 503 bila LISTEN belum atau sudah tidak tersambung, 200 bila tersambung.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		// LISTEN tidak aktif: balas 503 agar orkestrasi tahu service tidak sehat.
		if !listening.Load() {
			http.Error(w, "not listening", http.StatusServiceUnavailable)
			return
		}
		// LISTEN aktif: balas 200 dengan status ok.
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// Server health di port 8091 dengan batas waktu membaca header 5 detik.
	srv := &http.Server{Addr: ":8091", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	// Jalankan server health di goroutine terpisah agar listener utama bisa jalan.
	go func() {
		// Galat selain penutupan normal dicatat sebagai error.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server", "err", err)
		}
	}()

	// Rangkai service: store membaca database, dan push mengirim ke browser dengan kunci VAPID.
	svc := notify.NewService(
		store.New(pool),
		push.New(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDSubject),
	)

	// Hitung proses kirim yang masih berjalan, supaya shutdown bisa menunggunya.
	var inflight sync.WaitGroup
	// handle menjalankan satu event di goroutine sendiri agar listener tidak tertahan oleh pengiriman.
	handle := func(e event.Event) {
		// Tandai satu proses kirim yang sedang berjalan.
		inflight.Add(1)
		go func() {
			// Tandai selesai saat goroutine berakhir.
			defer inflight.Done()
			// Beri setiap event batas waktu 20 detik.
			sendCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			// Proses event. Galat hanya dicatat, tidak menghentikan listener.
			if err := svc.Handle(sendCtx, e); err != nil {
				slog.Error("handle event", "kind", e.Kind, "ticket_id", e.TicketID, "err", err)
			}
		}()
	}

	// Koneksi putus: sambung ulang. Sinyal selama putus hilang (outbox menyusul).
	// Ulangi selama konteks belum batal.
	for ctx.Err() == nil {
		// Jalankan listener sampai koneksi putus. Status listening diperbarui lewat callback.
		err := event.Listen(ctx, cfg.DatabaseURL, handle, func(up bool) { listening.Store(up) })
		// Listener berhenti: health harus menunjukkan tidak tersambung.
		listening.Store(false)
		// Berhenti karena galat (bukan karena shutdown): tunggu 3 detik lalu sambung ulang.
		if err != nil && ctx.Err() == nil {
			slog.Warn("listener stopped, reconnecting in 3s", "err", err)
			// Tunggu 3 detik, atau berhenti lebih cepat bila shutdown dimulai.
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
			}
		}
	}

	// Konteks batal: mulai shutdown dan tunggu proses kirim yang masih berjalan.
	slog.Info("shutting down, waiting for in-flight pushes")
	// Channel yang ditutup setelah semua proses kirim selesai.
	done := make(chan struct{})
	go func() { inflight.Wait(); close(done) }()
	// Tunggu sampai semua selesai, atau berhenti setelah 10 detik.
	select {
	// Semua proses kirim selesai.
	case <-done:
	// Waktu habis: catat bahwa proses kirim yang belum selesai dibatalkan.
	case <-time.After(10 * time.Second):
		slog.Warn("shutdown timeout, dropping in-flight pushes")
	}
	// Beri server health waktu 3 detik untuk berhenti dengan rapi.
	shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// Hentikan server health. Galatnya diabaikan karena proses akan berakhir.
	_ = srv.Shutdown(shCtx)
}
