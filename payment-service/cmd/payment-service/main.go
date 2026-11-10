// payment-service: memperpanjang langganan satu bulan setelah pembayaran Stripe berhasil; pembayaran pertama
// sebuah user juga menjadikannya owner.
// Sengaja terpisah dari backend: backend hanya MEMBACA owners.subscription_expires_at dan tidak pernah
// memanggil layanan ini, jadi bila payment-service mati antrian tetap berjalan; hanya perpanjangan yang tertunda.
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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"qms/payment-service/internal/billing"
	"qms/payment-service/internal/config"
	"qms/payment-service/internal/httpapi"
)

// fatal mencatat galat dengan pesan msg lalu menghentikan proses dengan kode 1.
func fatal(msg string, err error) {
	// Catat galat dalam format JSON sebelum proses berhenti.
	slog.Error(msg, "err", err)
	// Hentikan proses dengan status gagal.
	os.Exit(1)
}

// main membaca konfigurasi, memilih penyedia pembayaran (Stripe, simulator dev, atau nonaktif), lalu menjalankan API.
func main() {
	// Muat .env di folder akar proyek (../.env, relatif terhadap folder payment-service/). Di container tidak ada file ini.
	_ = godotenv.Load("../.env")
	// Baca dan validasi konfigurasi. Galat berarti DATABASE_URL, JWT_PUBLIC_KEY_FILE, atau QMS_PUBLIC_ORIGIN belum benar.
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	// Konteks yang batal saat SIGINT atau SIGTERM diterima.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// pgxpool.New tidak langsung menyambung: database yang belum siap tidak membuat service gagal start,
	// permintaan baru berhasil setelah database pulih.
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	// Galat konfigurasi pool (bukan galat koneksi): hentikan.
	if err != nil {
		fatal("database config", err)
	}
	// Tutup pool saat main selesai.
	defer pool.Close()

	// Penyedia pembayaran. Nil berarti pembayaran dinonaktifkan.
	var provider billing.Provider
	// Simulator dev aktif: pembayaran palsu. Dicatat keras di log.
	if cfg.DevSimulator {
		slog.Warn("DEV_PAYMENT_SIMULATOR is ON: payments are FAKE (no Stripe, no money). Never enable in production.")
		// Kunci Stripe sungguhan terisi: buat penyedia Stripe.
	} else if cfg.StripeConfigured() {
		provider = billing.NewStripeProvider(cfg.StripeKey, cfg.StripeBaseURL, cfg.WebhookSecret, cfg.PriceID, cfg.Amount, cfg.Currency)
		// Tidak ada penyedia yang dikonfigurasi: catat galat, tetapi jangan hentikan proses.
	} else {
		// Tidak fatal: service tetap hidup supaya salah konfigurasi di sini tidak mengganggu yang lain.
		slog.Error("STRIPE_API_KEY / STRIPE_WEBHOOK_SECRET not configured: payments disabled, checkout returns 503")
	}
	// Rangkai service billing dengan store PostgreSQL, penyedia, dan origin publik.
	svc := billing.New(billing.NewPGStore(pool), provider, cfg.PublicOrigin, slog.Default())
	// Simulator aktif: pasang harga dan mata uang palsu ke service.
	if cfg.DevSimulator {
		svc.WithSimulator(cfg.Amount, cfg.Currency)
	}

	// Server HTTP dengan batas waktu membaca header 10 detik, membaca dan menulis 30 detik, dan koneksi diam 60 detik.
	srv := &http.Server{
		// Alamat dan port dari konfigurasi.
		Addr: ":" + cfg.Port,
		// Handler API yang memakai service billing dan kunci publik JWT untuk memverifikasi login secara lokal.
		Handler: httpapi.New(svc, cfg.JWTPublicKey, cfg.JWTLegacySecret, slog.Default()).Handler(),
		// Batas waktu membaca header request.
		ReadHeaderTimeout: 10 * time.Second,
		// Batas waktu membaca seluruh request.
		ReadTimeout: 30 * time.Second,
		// Batas waktu menulis respons.
		WriteTimeout: 30 * time.Second,
		// Batas koneksi keep-alive yang diam.
		IdleTimeout: 60 * time.Second,
	}
	// Jalankan server di goroutine terpisah agar main bisa menunggu sinyal berhenti.
	go func() {
		// Catat port, status pembayaran, mode simulator, dan URL Stripe saat server mulai.
		slog.Info("payment-service starting", "port", cfg.Port, "enabled", svc.Enabled(), "simulator", cfg.DevSimulator, "stripe_base_url", cfg.StripeBaseURL)
		// Server berhenti karena galat selain penutupan normal: hentikan proses.
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("server", err)
		}
	}()

	// Tunggu sampai ada sinyal berhenti.
	<-ctx.Done()
	// Beri server 10 detik untuk menyelesaikan request yang sedang berjalan.
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Hentikan server. Galatnya diabaikan karena proses akan berakhir.
	_ = srv.Shutdown(shutdown)
}
