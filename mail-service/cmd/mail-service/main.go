// mail-service: satu-satunya pengirim email di QMS. Menerima permintaan lewat API internal bertoken, mengisi
// templat, dan mengirim lewat SMTP (Mailpit di dev, SMTP nyata di produksi). Bila DATABASE_URL diisi, worker
// outbox juga mengirim email dari tabel notification_outbox.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qms/mail-service/internal/config"
	"qms/mail-service/internal/handler"
	"qms/mail-service/internal/logging"
	"qms/mail-service/internal/mailer"
	"qms/mail-service/internal/mailtemplate"
	"qms/mail-service/internal/metrics"
	"qms/mail-service/internal/middleware"
	"qms/mail-service/internal/repository"
	"qms/mail-service/internal/router"
	"qms/mail-service/internal/service"
	"qms/mail-service/internal/telemetry"
)

// main menjalankan run, lalu mencatat galatnya dan keluar dengan kode 1 bila gagal.
func main() {
	// Atur logger default ke JSON di stdout, dengan atribut service=mail-service dan level dari LOG_LEVEL.
	slog.SetDefault(
		logging.NewLogger(
			"mail-service",
			os.Getenv("LOG_LEVEL"),
		),
	)

	// Jalankan perintah atau layanan. Galat berarti proses harus berhenti dengan status gagal.
	if err := run(); err != nil {
		// Catat galat dalam format JSON sebelum proses berhenti.
		slog.Error("mail-service stopped", "err", err)
		// Hentikan proses dengan status gagal.
		os.Exit(1)
	}
}

// run menjalankan perintah send-test bila diminta, atau menyalakan API dan worker outbox sampai ada sinyal berhenti.
func run() error {
	// Argumen pertama "send-test": jalankan perintah uji email, lalu keluar tanpa menyalakan server.
	if len(os.Args) > 1 && os.Args[1] == "send-test" {
		// Perintah harus tepat dengan satu alamat tujuan.
		if len(os.Args) != 3 {
			// Alamat tujuan wajib persis satu argumen.
			return errors.New("usage: mail-service send-test <alamat-email>")
		}
		// Kirim satu email uji dan selesai.
		return sendTest(os.Args[2])
	}

	// Mode layanan: baca konfigurasi. Galat berarti token atau SMTP belum benar.
	cfg, err := config.Load()
	// Konfigurasi salah: gagal cepat sebelum menerima permintaan apa pun.
	if err != nil {
		// Awalan config memudahkan melihat asal galat di log.
		return fmt.Errorf("config: %w", err)
	}
	// tracing: aktif hanya bila OTEL_EXPORTER_OTLP_ENDPOINT terisi; kosong berarti no-op. Sisa span dikirim saat
	// berhenti (maks 5 detik). Pengiriman asinkron dan terbatas, jadi collector mati tidak pernah menahan request.
	shutdownTracing := telemetry.Setup(context.Background(), "mail-service")
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(sctx)
	}()

	// Rangkai lapisan: pengirim SMTP, pengisi templat, service, handler, dan router.
	renderer := mailtemplate.New()
	// Mailer dan service dibuat sekali dan dipakai bersama oleh semua permintaan.
	mailSvc := service.NewMailService(
		mailer.New(cfg.SMTP),
		renderer,
		slog.Default(),
	)
	// Metrik Prometheus: middleware HTTP dipasang di router, metrik outbox diisi worker, listener dijalankan nanti.
	appMetrics := metrics.NewMail()
	// Router mendaftarkan rute kirim yang dilindungi token, lalu rute health dan 404.
	appRouter := router.NewAppRouter(
		handler.NewMailHandler(mailSvc),
		middleware.ServiceToken(cfg.ServiceToken),
	).WithSystem(handler.NewSystemHandler()).WithMetrics(appMetrics.Middleware())
	// tracing hanya dipasang bila aktif; tanpa itu tidak ada middleware tambahan
	if telemetry.Enabled() {
		appRouter.WithTracing(telemetry.Middleware("mail-service"))
	}
	// Server HTTP dengan batas waktu baca, tulis (60 detik karena kirim sinkron bisa lama), dan koneksi diam.
	srv := &http.Server{
		// Alamat dan port dari konfigurasi.
		Addr: ":" + cfg.Port,
		// Handler API.
		Handler: router.SetupRouter(appRouter),
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
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	// Lepas pendaftaran sinyal saat run selesai.
	defer stop()
	// Listener metrik di port terpisah (METRICS_ADDR, bawaan :9100); tidak memblokir dan tidak menggagalkan service.
	appMetrics.Serve(ctx)
	// Outbox aktif bila DATABASE_URL diisi: jalankan worker di goroutine terpisah.
	if cfg.Outbox.Enabled() {
		// Worker outbox jalan terpisah agar tidak memblokir API.
		go runOutbox(ctx, cfg, renderer, appMetrics)
	} else {
		// Tanpa database, worker outbox tidak jalan; layanan hanya melayani API kirim langsung.
		slog.Warn("DATABASE_URL not set: outbox worker disabled, only POST /internal/v1/send is served")
	}
	// Jalankan server HTTP di goroutine terpisah. Galat selain penutupan normal dikirim lewat saluran.
	serverErr := make(chan error, 1)
	// ListenAndServe memblokir, jadi dijalankan di goroutine sementara run menunggu sinyal.
	go func() {
		// Catat alamat, SMTP, dan mode TLS saat server mulai mendengarkan.
		slog.Info("mail-service listening", "addr", srv.Addr, "smtp", cfg.SMTP.Addr(), "tls", cfg.SMTP.TLS)
		// Galat selain penutupan normal: laporkan agar proses berhenti.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server: %w", err)
		}
	}()
	// Tunggu sampai ada sinyal berhenti atau server gagal.
	select {
	// Server gagal start atau berhenti tiba-tiba: kembalikan galatnya.
	case err := <-serverErr:
		return err
	// Sinyal berhenti diterima: lanjut ke shutdown.
	case <-ctx.Done():
	}

	// Mulai shutdown: beri email yang sedang dikirim waktu sampai 30 detik untuk selesai.
	slog.Info("shutting down, waiting for in-flight emails")
	// Server berhenti menerima koneksi baru, tetapi permintaan yang berjalan diberi waktu selesai.
	shCtx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	// Lepas timer saat run selesai.
	defer cancel()
	// Hentikan server. Galatnya diabaikan karena proses akan berakhir.
	_ = srv.Shutdown(shCtx)
	// Shutdown selesai.
	return nil
}

// sendTest: `mail-service send-test <alamat>` mengirim satu email uji lewat SMTP yang dikonfigurasi lalu keluar.
// Hanya butuh SMTP_*; dipakai memeriksa jalur email tanpa lewat API.
func sendTest(
	to string,
) error {
	// Baca konfigurasi SMTP. Galat berarti SMTP_* belum lengkap atau tidak valid.
	smtpCfg, err := config.LoadSMTP()
	if err != nil {
		// Awalan config memudahkan melihat asal galat di log.
		return fmt.Errorf("config: %w", err)
	}
	// Rangkai service dengan pengirim SMTP dan pengisi templat, lalu kirim email uji.
	svc := service.NewMailService(
		mailer.New(smtpCfg),
		mailtemplate.New(),
		slog.Default(),
	)
	// Kirim email uji.
	if err := svc.SendTest(context.Background(), to); err != nil {
		// Teruskan galat SMTP apa adanya agar penyebab terlihat.
		return err
	}
	// Berhasil: catat alamat SMTP yang dipakai.
	slog.Info("test email sent", "smtp", smtpCfg.Addr())
	// Selesai tanpa galat.
	return nil
}

// runOutbox menjalankan worker outbox. Kegagalan menyambung database tidak mematikan layanan (API kirim tetap
// jalan); sambungan dicoba lagi sampai berhasil.
func runOutbox(
	ctx context.Context,
	cfg config.Config,
	renderer service.TemplateRenderer,
	appMetrics *metrics.Mail,
) {
	// Sambung ke database outbox, diulang sampai berhasil. Nil bila konteks batal lebih dulu.
	db := repository.ConnectWithRetry(
		ctx,
		cfg.Outbox.DatabaseURL,
	)
	if db == nil {
		// Shutdown dimulai sebelum database tersambung: tidak ada yang dijalankan.
		return
	}
	// Tutup koneksi saat worker berhenti.
	defer db.Close()
	// Repository outbox dipakai worker dan gauge mail_outbox_pending (dihitung tiap scrape).
	outboxRepo := repository.NewOutboxRepository(db)
	appMetrics.SetPending(outboxRepo.Pending)
	// Rangkai worker: repository PostgreSQL (dibungkus penghitung hasil kirim), pengirim SMTP, pengisi templat, dan
	// konfigurasi outbox.
	w := service.NewOutboxService(
		service.NewMeteredOutboxStore(outboxRepo, appMetrics),
		mailer.New(cfg.SMTP),
		renderer,
		service.OutboxConfig{
			PublicOrigin: cfg.Outbox.PublicOrigin,
			Rate:         cfg.Outbox.Rate,
			MaxAttempts:  cfg.Outbox.MaxAttempts,
		},
		slog.Default(),
	)
	// Dengarkan NOTIFY mail_outbox di goroutine terpisah untuk membangunkan worker saat ada baris baru.
	go repository.Listen(
		ctx,
		cfg.Outbox.DatabaseURL,
		w.Wake,
		slog.Default(),
	)
	// Catat laju dan batas percobaan yang aktif.
	slog.Info(
		"outbox worker started",
		"rate_per_sec",
		cfg.Outbox.Rate,
		"max_attempts",
		cfg.Outbox.MaxAttempts,
	)
	// Jalankan worker sampai konteks batal.
	w.Run(ctx)
}
