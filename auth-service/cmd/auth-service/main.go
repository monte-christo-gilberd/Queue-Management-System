// auth-service: login, registrasi, reset password, dan verifikasi email QMS. Sengaja terpisah dari backend:
// backend memverifikasi JWT RS256 secara lokal dengan kunci publik dan tidak pernah memanggil layanan ini, jadi bila
// auth-service mati, user yang sudah login tetap bisa memakai aplikasi; hanya login baru, registrasi, reset password,
// dan verifikasi email yang tertunda (nginx menjawab 503 AUTH_UNAVAILABLE).
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

	"qms/auth-service/internal/auth"
	"qms/auth-service/internal/config"
	"qms/auth-service/internal/google"
	"qms/auth-service/internal/httpapi"
	"qms/auth-service/internal/mailclient"
	"qms/auth-service/internal/store"
)

// fatal mencatat galat lalu menghentikan proses dengan kode 1.
func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}

// main membaca konfigurasi, membuka database, merangkai service dan router, lalu menjalankan server sampai
// menerima SIGINT atau SIGTERM.
func main() {
	// baca dan validasi konfigurasi; galat berarti server tidak boleh jalan
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	// buka koneksi database dengan role auth_rw dan pastikan terjangkau
	db, err := store.Connect(cfg.DatabaseURL)
	if err != nil {
		fatal("database", err)
	}
	// tutup koneksi setelah server berhenti
	defer db.Close()

	// pengirim email opsional: tanpa mail-service, reset password dan verifikasi tidak dipasang
	var mailer auth.Mailer
	if cfg.MailServiceURL != "" {
		mailer = mailclient.New(cfg.MailServiceURL, cfg.MailServiceToken, 0)
	} else {
		slog.Warn("MAIL_SERVICE_URL kosong: lupa/reset password dan verifikasi email tidak dipasang")
	}

	// rangkai logika akun dari bawah ke atas: penyimpanan, penerbit token, pengirim email
	svc := auth.NewService(
		store.NewUserStore(db),
		store.NewTokenStore(db),
		auth.NewTokenIssuer(cfg.JWTPrivateKey, cfg.JWTKeyID),
		mailer,
		auth.Options{PublicAppURL: cfg.PublicAppURL, VerificationRequired: cfg.EmailVerificationRequired},
	)

	// login Google opsional: hanya menyala bila GOOGLE_CLIENT_IDS terisi
	if len(cfg.GoogleClientIDs) > 0 {
		svc.WithGoogle(google.NewVerifier(cfg.GoogleCertsURL, cfg.GoogleClientIDs))
	} else {
		slog.Warn("GOOGLE_CLIENT_IDS kosong: login Google tidak dipasang")
	}

	// pasang aturan validasi no_html_tags sebelum request pertama
	httpapi.RegisterValidators()
	// server HTTP dengan batas waktu agar koneksi macet tidak menggantung
	srv := &http.Server{
		Addr: ":" + cfg.Port,
		// Hanya nginx di jaringan internal yang boleh menentukan IP klien (X-Forwarded-For).
		Handler:           httpapi.NewRouter(svc, mailer != nil, trustedProxies()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// context yang batal saat SIGINT atau SIGTERM diterima
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// jalankan server di goroutine agar main bisa menunggu sinyal berhenti
	go func() {
		slog.Info("auth-service starting", "port", cfg.Port, "email", mailer != nil, "verification", cfg.EmailVerificationRequired, "google", svc.GoogleEnabled())
		// berhenti karena galat selain penutupan normal: hentikan proses
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("server", err)
		}
	}()

	// tunggu sinyal berhenti, lalu beri request yang sedang berjalan waktu untuk selesai
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

// trustedProxies membaca TRUSTED_PROXIES (daftar CIDR dipisah koma); kosong = tidak ada proxy yang dipercaya.
func trustedProxies() []string {
	raw := os.Getenv("TRUSTED_PROXIES")
	var out []string
	start := 0
	// pecah di setiap koma, buang spasi dan entri kosong
	for i := 0; i <= len(raw); i++ {
		if i == len(raw) || raw[i] == ',' {
			if p := trimSpace(raw[start:i]); p != "" {
				out = append(out, p)
			}
			start = i + 1
		}
	}
	return out
}

// trimSpace membuang spasi dan tab di kedua ujung s.
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
