// mail-service: satu-satunya pengirim email di QMS. Stateless dan tanpa database: menerima permintaan lewat
// API internal bertoken, mengisi templat, dan mengirim lewat SMTP (Mailpit di dev, SMTP nyata di produksi).
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qms/mail-service/internal/config"
	"qms/mail-service/internal/httpapi"
	"qms/mail-service/internal/mailer"
	"qms/mail-service/internal/outbox"

	"github.com/jackc/pgx/v5/pgxpool"
)

// fatal mencatat galat dengan pesan msg lalu menghentikan proses dengan kode 1.
func fatal(msg string, err error) {
	// Catat galat dalam format JSON sebelum proses berhenti.
	slog.Error(msg, "err", err)
	// Hentikan proses dengan status gagal.
	os.Exit(1)
}

// sendTest: `mail-service send-test <alamat>` mengirim satu email uji lewat SMTP yang dikonfigurasi lalu keluar.
// Hanya butuh SMTP_*; dipakai memeriksa jalur email tanpa lewat API.
func sendTest(to string) {
	// Baca konfigurasi SMTP. Galat berarti SMTP_* belum lengkap atau tidak valid.
	smtpCfg, err := config.LoadSMTP()
	if err != nil {
		fatal("config", err)
	}
	// Isi templat "test" dengan alamat tujuan, waktu kirim, dan tautan contoh.
	r, err := mailer.Render("test", map[string]any{
		"To":     to,
		"SentAt": time.Now().Format("2 Jan 2006 15:04:05 MST"),
		"Link":   "http://localhost:8080/reset-password?token=contoh-token-uji",
	})
	// Templat gagal diisi: hentikan.
	if err != nil {
		fatal("render", err)
	}
	// Beri pengiriman batas waktu 30 detik.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Kirim email uji. Galat berarti SMTP tidak bisa dipakai, jadi hentikan dengan status gagal.
	if err := mailer.New(smtpCfg).Send(ctx, mailer.Message{To: to, Subject: r.Subject, Text: r.Text, HTML: r.HTML}); err != nil {
		fatal("send test email", err)
	}
	// Berhasil: catat alamat SMTP yang dipakai.
	slog.Info("test email sent", "smtp", smtpCfg.Addr())
}

// runOutbox menjalankan worker outbox. Kegagalan menyambung database tidak mematikan layanan (API kirim tetap
// jalan); worker dicoba lagi sampai berhasil.
func runOutbox(ctx context.Context, cfg config.Config) {
	// Pool koneksi ke database outbox. Nil selama belum berhasil tersambung.
	var pool *pgxpool.Pool
	// Ulangi sambungan selama belum berhasil dan konteks belum batal.
	for pool == nil && ctx.Err() == nil {
		// Buat pool, lalu ping untuk memastikan database benar-benar bisa dijangkau.
		p, err := pgxpool.New(ctx, cfg.Outbox.DatabaseURL)
		if err == nil {
			err = p.Ping(ctx)
		}
		// Gagal tersambung: catat, tunggu 5 detik, lalu coba lagi (kecuali shutdown dimulai).
		if err != nil {
			slog.Error("outbox database not reachable, retrying", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		// Berhasil tersambung: simpan pool.
		pool = p
	}
	// Konteks batal sebelum berhasil tersambung: tidak ada yang perlu dijalankan.
	if pool == nil {
		return
	}
	// Tutup pool saat worker berhenti.
	defer pool.Close()
	// Rangkai worker: store PostgreSQL, pengirim SMTP, dan konfigurasi outbox.
	w := outbox.NewWorker(outbox.NewPG(pool), mailer.New(cfg.SMTP), outbox.Config{
		PublicOrigin: cfg.Outbox.PublicOrigin,
		Rate:         cfg.Outbox.Rate,
		MaxAttempts:  cfg.Outbox.MaxAttempts,
	}, slog.Default())
	// Dengarkan NOTIFY mail_outbox di goroutine terpisah untuk membangunkan worker saat ada baris baru.
	go outbox.Listen(ctx, pool, w.Wake, slog.Default())
	// Catat laju dan batas percobaan yang aktif.
	slog.Info("outbox worker started", "rate_per_sec", cfg.Outbox.Rate, "max_attempts", cfg.Outbox.MaxAttempts)
	// Jalankan worker sampai konteks batal.
	w.Run(ctx)
}

// main menjalankan perintah send-test bila diminta, atau menyalakan API dan worker outbox.
func main() {
	// Atur logger default ke JSON di stdout.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Argumen pertama "send-test": jalankan perintah uji email, lalu keluar tanpa menyalakan server.
	if len(os.Args) > 1 && os.Args[1] == "send-test" {
		// Perintah harus tepat dengan satu alamat tujuan.
		if len(os.Args) != 3 {
			fatal("usage", errors.New("mail-service send-test <alamat-email>"))
		}
		sendTest(os.Args[2])
		return
	}

	// Mode layanan: baca konfigurasi. Galat berarti token atau SMTP belum benar.
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	// Rangkai API dengan Mailer, token layanan, dan logger.
	api := httpapi.New(mailer.New(cfg.SMTP), cfg.ServiceToken, slog.Default())
	// Server HTTP dengan batas waktu baca, tulis (60 detik karena kirim sinkron bisa lama), dan koneksi diam.
	srv := &http.Server{
		// Alamat dan port dari konfigurasi.
		Addr: ":" + cfg.Port,
		// Handler API.
		Handler: api.Handler(),
		// Batas waktu membaca header request (5 detik).
		ReadHeaderTimeout: 5 * time.Second,
		// Batas waktu membaca seluruh request (10 detik).
		ReadTimeout: 10 * time.Second,
		// Batas waktu menulis respons (60 detik): pengiriman sinkron dengan coba ulang bisa sampai sekitar 45 detik.
		WriteTimeout: 60 * time.Second,
		// Batas koneksi keep-alive yang diam (60 detik).
		IdleTimeout: 60 * time.Second,
	}

	// Konteks yang batal saat SIGINT atau SIGTERM diterima.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// Outbox aktif bila DATABASE_URL diisi: jalankan worker di goroutine terpisah.
	if cfg.Outbox.Enabled() {
		go runOutbox(ctx, cfg)
	} else {
		// Tanpa database, worker outbox tidak jalan; layanan hanya melayani API kirim langsung.
		slog.Warn("DATABASE_URL not set: outbox worker disabled, only POST /internal/v1/send is served")
	}
	// Jalankan server HTTP di goroutine terpisah. Galat selain penutupan normal menghentikan proses.
	go func() {
		// Catat alamat, SMTP, dan mode TLS saat server mulai mendengarkan.
		slog.Info("mail-service listening", "addr", srv.Addr, "smtp", cfg.SMTP.Addr(), "tls", cfg.SMTP.TLS)
		// Galat selain penutupan normal: hentikan proses.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("server", err)
		}
	}()
	// Tunggu sampai ada sinyal berhenti.
	<-ctx.Done()

	// Mulai shutdown: beri email yang sedang dikirim waktu sampai 30 detik untuk selesai.
	slog.Info("shutting down, waiting for in-flight emails")
	shCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Hentikan server. Galatnya diabaikan karena proses akan berakhir.
	_ = srv.Shutdown(shCtx)
}
