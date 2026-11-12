// auth-service: login, registrasi, reset password, dan verifikasi email QMS. Sengaja terpisah dari backend:
// backend memverifikasi JWT RS256 secara lokal dengan kunci publik dan tidak pernah memanggil layanan ini, jadi bila
// auth-service mati, user yang sudah login tetap bisa memakai aplikasi; hanya login baru, registrasi, reset password,
// dan verifikasi email yang tertunda (nginx menjawab 503 AUTH_UNAVAILABLE).
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

	"qms/auth-service/internal/config"
	"qms/auth-service/internal/database"
	"qms/auth-service/internal/google"
	"qms/auth-service/internal/handler"
	"qms/auth-service/internal/httpserver"
	"qms/auth-service/internal/logging"
	"qms/auth-service/internal/mailclient"
	"qms/auth-service/internal/metrics"
	"qms/auth-service/internal/middleware"
	"qms/auth-service/internal/repository"
	"qms/auth-service/internal/router"
	"qms/auth-service/internal/service"
	"qms/auth-service/internal/validator"
)

// main hanya memanggil run dan mencatat error-nya. Pekerjaan sebenarnya ada di run agar defer di dalamnya
// (termasuk db.Close) sudah berjalan sebelum proses berhenti dengan kode 1.
func main() {
	// logger JSON dengan atribut service=auth-service dan level dari LOG_LEVEL (bawaan info)
	logger := logging.NewLogger(
		"auth-service",
		os.Getenv("LOG_LEVEL"),
	)
	// jadikan default: slog.* di seluruh paket (service, handler, middleware) ikut berformat JSON
	slog.SetDefault(logger)

	// jalankan service; galat berarti server gagal start atau berhenti tidak wajar
	if err := run(); err != nil {
		// Catat penyebab berhenti sebagai log terstruktur.
		slog.Error("auth-service stopped", "err", err)
		// Keluar dengan kode 1 agar orkestrator (docker) tahu service gagal.
		os.Exit(1)
	}
}

// run membaca konfigurasi, membuka database, merangkai repository, service, handler, dan router dari bawah ke atas,
// lalu menjalankan server sampai menerima SIGINT atau SIGTERM.
func run() error {
	// baca dan validasi konfigurasi; galat berarti server tidak boleh jalan
	cfg, err := config.Load()
	if err != nil {
		// Bungkus dengan asal galat agar log jelas.
		return fmt.Errorf("config: %w", err)
	}
	// buka koneksi database dengan role auth_rw dan pastikan terjangkau
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		// Bungkus dengan asal galat agar log jelas.
		return fmt.Errorf("database: %w", err)
	}
	// tutup koneksi setelah server berhenti
	defer db.Close()

	// pengirim email opsional: tanpa mail-service, reset password dan verifikasi tidak dipasang
	var mailer service.Mailer
	if cfg.MailServiceURL != "" {
		// Timeout 0 berarti klien memakai batas bawaannya.
		mailer = mailclient.New(cfg.MailServiceURL, cfg.MailServiceToken, 0)
	} else {
		// Beri peringatan agar operator tahu fitur email nonaktif.
		slog.Warn("MAIL_SERVICE_URL kosong: lupa/reset password dan verifikasi email tidak dipasang")
	}

	// rangkai logika akun dari bawah ke atas: repository, penerbit token, pengirim email
	svc := service.NewService(
		repository.NewUserRepository(db),
		repository.NewTokenRepository(db),
		service.NewTokenIssuer(cfg.JWTPrivateKey, cfg.JWTKeyID),
		mailer,
		service.Options{PublicAppURL: cfg.PublicAppURL, VerificationRequired: cfg.EmailVerificationRequired},
	)

	// login Google opsional: hanya menyala bila GOOGLE_CLIENT_IDS terisi
	if len(cfg.GoogleClientIDs) > 0 {
		// Verifier memeriksa ID token Google untuk client ID yang dikonfigurasi.
		svc.WithGoogle(google.NewVerifier(cfg.GoogleCertsURL, cfg.GoogleClientIDs))
	} else {
		// Beri peringatan agar operator tahu login Google nonaktif.
		slog.Warn("GOOGLE_CLIENT_IDS kosong: login Google tidak dipasang")
	}

	// pasang aturan validasi no_html_tags sebelum request pertama
	validator.RegisterValidators()
	// handler akun satu-satunya; dipakai untuk semua rute
	authHandler := handler.NewAuthHandler(svc)
	// rangkai router: registrasi dan login wajib, hanya proxy tepercaya yang boleh menentukan IP klien
	app := router.NewAppRouter(
		authHandler,
		middleware.Auth(svc),
		cfg.TrustedProxies,
	)
	// metrik Prometheus: middleware HTTP dipasang di router, listener /metrics dijalankan setelah ctx dibuat
	appMetrics := metrics.NewAuth()
	app.WithMetrics(appMetrics.Middleware())
	// reset password dan verifikasi email hanya dipasang bila mail-service terpasang
	if mailer != nil {
		// Pasang rute lupa password dan verifikasi email.
		app.WithEmail(authHandler)
	}
	// rute login Google hanya dipasang bila Google dinyalakan
	if svc.GoogleEnabled() {
		// Pasang rute login Google.
		app.WithGoogle(authHandler)
	}
	// server HTTP dengan batas waktu agar koneksi macet tidak menggantung
	srv := httpserver.New(":"+cfg.Port, router.SetupRouter(app))

	// context yang batal saat SIGINT atau SIGTERM diterima
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	// Lepas pendaftaran sinyal saat run selesai.
	defer stop()
	// listener metrik di port terpisah (METRICS_ADDR, bawaan :9100); tidak memblokir dan tidak menggagalkan service
	appMetrics.Serve(ctx)
	// jalankan server di goroutine agar run bisa menunggu sinyal berhenti
	serverErr := make(chan error, 1)
	// Server berjalan di goroutine terpisah.
	go func() {
		// Catat konfigurasi efektif (tanpa rahasia) saat start.
		slog.Info("auth-service starting", "port", cfg.Port, "email", mailer != nil, "verification", cfg.EmailVerificationRequired, "google", svc.GoogleEnabled())
		// berhenti karena galat selain penutupan normal: laporkan ke run
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Kirim galat ke run.
			serverErr <- err
		}
	}()

	// tunggu sinyal berhenti atau galat server
	select {
	// Server berhenti sendiri karena galat.
	case err := <-serverErr:
		// Bungkus dengan asal galat.
		return fmt.Errorf("server: %w", err)
	// Sinyal diterima: lanjut ke penutupan bertahap.
	case <-ctx.Done():
	}
	// beri request yang sedang berjalan waktu untuk selesai
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	// Lepas timer saat run selesai.
	defer cancel()
	// Berhenti menerima koneksi baru dan menunggu request berjalan selesai; galat diabaikan karena proses akan berhenti.
	_ = srv.Shutdown(shutdown)
	return nil
}
