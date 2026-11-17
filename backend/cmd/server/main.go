// Command server adalah API utama Queue Management System: sistem antrian untuk layanan tatap muka.
//
// Owner membuat antrian, memanggil nomor berikutnya, dan mengelola counter serta layar publik. Customer bergabung
// lewat tautan atau QR, lalu melihat posisi dan nomor yang sedang dilayani secara real-time (SSE) tanpa refresh.
// Layar publik menampilkan nomor tanpa login lewat token di URL. Riwayat antrian dan tiket dapat dibaca kembali.
//
// Binary ini hanya menyambungkan komponen dan menjalankan HTTP server. Logika ada di internal/service,
// akses data di internal/repository, dan endpoint di internal/handler. Login, registrasi, reset password, dan
// verifikasi email ada di auth-service (backend hanya memverifikasi tokennya secara lokal, tanpa memanggilnya).
// Satu-satunya layanan yang dipanggil backend lewat HTTP adalah payment-service (checkout langganan); layanan notifikasi dan
// email berjalan terpisah dan bekerja lewat database. Kode join QR dibuat dan diverifikasi di backend sendiri.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qms/internal/app"
	"qms/internal/config"
	"qms/internal/database"
	"qms/internal/httpserver"
	"qms/internal/logging"
	"qms/internal/telemetry"
)

// main memasang logger JSON, lalu memanggil run dan mencatat error-nya. Pekerjaan sebenarnya ada di run agar defer
// di dalamnya (termasuk db.Close) sudah berjalan sebelum proses berhenti dengan kode 1.
func main() {
	// logger JSON dengan atribut service=backend dan level dari LOG_LEVEL (bawaan info)
	logger := logging.NewLogger(
		"backend",
		os.Getenv("LOG_LEVEL"),
	)
	// jadikan default: semua slog.* di proses ini, dan paket log bawaan Go milik pustaka, ikut berformat JSON
	slog.SetDefault(logger)

	// jalankan service; galat berarti gagal start atau berhenti tidak wajar
	if err := run(); err != nil {
		// catat penyebab sebagai log terstruktur, lalu keluar dengan kode 1 agar Docker tahu service gagal
		slog.Error("backend stopped", "err", err)
		os.Exit(1)
	}
}

// run membaca config, membuka database, memanggil app.Wire (pemasangan dependensi) dan app.NewEngine (persiapan
// engine HTTP), lalu menjalankan server sampai proses menerima SIGINT atau SIGTERM.
//
// Berhenti dengan rapi: koneksi SSE ditutup, request yang sedang berjalan dituntaskan (paling lama
// SHUTDOWN_TIMEOUT), lalu koneksi database ditutup oleh defer. Kembali dengan error bila config, database, atau
// engine gagal disiapkan, atau bila server gagal start atau shutdown tidak selesai tepat waktu.
func run() error {
	// membaca variabel env sebagai config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// tracing: aktif hanya bila OTEL_EXPORTER_OTLP_ENDPOINT terisi; kosong berarti no-op. Dipasang sebelum database
	// dibuka karena pembungkus query memeriksa telemetry.Enabled(). Sisa span dikirim saat berhenti (maks 5 detik).
	shutdownTracing := telemetry.Setup(context.Background(), "backend")
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(sctx)
	}()

	// membuka koneksi ke database dan cek koneksi (Ping)
	db, err := database.Connect(
		database.Config{
			DSN: cfg.DatabaseURL,
		},
	)

	if err != nil {
		return err
	}

	// menutup koneksi database saat run selesai, yaitu setelah server berhenti dan request terakhir dituntaskan
	defer db.Close()

	// memasang semua dependensi dan menjalankan hub SSE
	application := app.Wire(
		&cfg,
		db,
	)

	// membuat engine gin dan mendaftarkan semua rute
	engine, err := app.NewEngine(
		&cfg,
		application.Router,
	)

	if err != nil {
		return err
	}

	// membuat string port dari config
	port := fmt.Sprintf(":%s", cfg.Port)

	// ctx dibatalkan saat proses menerima SIGINT (Ctrl+C) atau SIGTERM (docker stop)
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// listener metrik Prometheus di port terpisah (METRICS_ADDR, bawaan :9100). Tidak memblokir, dan kegagalannya
	// tidak menghentikan API.
	application.Metrics.Serve(ctx)

	// menjalankan server HTTP dengan timeout dasar; Hub.Shutdown menutup koneksi SSE saat server berhenti,
	// agar graceful shutdown tidak menunggu koneksi yang berumur panjang
	srv := httpserver.New(port, engine)
	slog.Info("server listening", "addr", port)

	err = httpserver.Run(
		ctx,
		srv,
		cfg.ShutdownTimeout,
		application.Hub.Shutdown,
	)

	if err == nil {
		slog.Info("server stopped gracefully")
	}
	return err
}
