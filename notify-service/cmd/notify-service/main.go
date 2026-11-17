// Command notify-service menjalankan layanan notifikasi push QMS: mendengarkan event dari database
// (LISTEN queue_events), menentukan penerima, lalu mengirim Web Push ke browser dan FCM ke aplikasi Android.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qms/notify-service/internal/config"
	"qms/notify-service/internal/fcm"
	"qms/notify-service/internal/httpserver"
	"qms/notify-service/internal/listener"
	"qms/notify-service/internal/logging"
	"qms/notify-service/internal/metrics"
	"qms/notify-service/internal/model"
	"qms/notify-service/internal/pusher"
	"qms/notify-service/internal/repository"
	"qms/notify-service/internal/runner"
	"qms/notify-service/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// healthAddr: alamat server health check.
const healthAddr = ":8091"

// main memanggil run, lalu keluar dengan kode 1 bila gagal. Error sudah dicatat di run.
func main() {
	// Atur logger default ke JSON di stdout untuk seluruh proses, dengan atribut service=notify-service
	// dan level dari LOG_LEVEL (bawaan info).
	slog.SetDefault(logging.NewLogger("notify-service", os.Getenv("LOG_LEVEL")))

	// Jalankan service. Error berarti service gagal start, jadi hentikan proses dengan status gagal.
	if err := run(); err != nil {
		os.Exit(1)
	}
}

// run menyusun konfigurasi, koneksi, health server, dan listener, lalu menunggu sinyal berhenti.
func run() error {
	// Baca dan validasi konfigurasi dari environment.
	cfg, err := config.Load()
	// Error berarti ada variabel wajib yang belum diisi, jadi service tidak boleh start.
	if err != nil {
		slog.Error("config", "err", err)
		return err
	}

	// Buat konteks yang batal saat SIGINT atau SIGTERM diterima.
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	defer stop()

	// Buat pool koneksi ke PostgreSQL (sqlx dengan driver pgx) untuk query baca dan hapus langganan. sqlx.Open
	// tidak menyambung saat dibuat, jadi database yang belum siap tidak menggagalkan start; listener LISTEN
	// menyambung ulang sendiri dan /health melaporkan statusnya.
	db, err := sqlx.Open("pgx", cfg.DatabaseURL)
	// Error berarti URL salah, jadi hentikan start.
	if err != nil {
		slog.Error("db pool", "err", err)
		return err
	}
	// Tutup pool saat run selesai.
	defer db.Close()

	// /health: 200 hanya bila LISTEN queue_events sedang tersambung.
	health := &httpserver.Health{}
	// Server health di port 8091.
	srv := httpserver.New(healthAddr, health)
	// Jalankan server health di goroutine terpisah agar listener utama bisa jalan.
	go func() {
		// Error selain penutupan normal dicatat sebagai error.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("health server", "err", err)
		}
	}()

	// Metrik Prometheus (push terkirim dan gagal). Listener /metrics di port terpisah (METRICS_ADDR, bawaan :9100);
	// tidak memblokir dan tidak menggagalkan service.
	appMetrics := metrics.NewNotify()
	appMetrics.Serve(ctx)

	// Rangkai service: repository membaca database, pusher mengirim ke browser dengan kunci VAPID, dan fcm
	// mengirim ke aplikasi Android bila kredensial FCM terpasang. Dispatcher memilih pengirim sesuai jenis
	// langganan. Pengirim dibungkus penghitung metrik tanpa mengubah perilakunya.
	repo := repository.NewNotifyRepository(db)
	fcmSender := newFCMSender(cfg)
	// FCM aktif: token di push_devices ikut dibaca. Bila mati, tabel itu tidak disentuh sama sekali.
	var fcmChannel pusher.ChannelSender
	if fcmSender != nil {
		repo.EnableDevices()
		fcmChannel = fcmSender
	}
	svc := service.NewNotifyService(
		repo,
		service.NewMeteredPushSender(
			pusher.NewDispatcher(
				pusher.New(
					cfg.VAPIDPublicKey,
					cfg.VAPIDPrivateKey,
					cfg.VAPIDSubject),
				fcmChannel,
			),
			appMetrics,
		),
	)

	// Rangkai runner: listener LISTEN dengan DSN dari konfigurasi, dan health diperbarui lewat callback.
	r := runner.New(
		svc,
		func(
			ctx context.Context,
			handle func(model.Event),
			onState func(bool),
		) error {
			// Jalankan listener dengan DSN dari konfigurasi.
			return listener.Listen(
				ctx,
				cfg.DatabaseURL,
				handle,
				onState,
			)
		},
		health.SetListening,
	)
	// Jalankan sampai konteks batal, lalu tunggu proses kirim yang masih berjalan.
	r.Run(ctx)

	// Beri server health waktu 3 detik untuk berhenti dengan rapi.
	shCtx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)

	defer cancel()
	// Hentikan server health. Errornya diabaikan karena proses akan berakhir.
	_ = srv.Shutdown(shCtx)
	// Selesai tanpa Error.
	return nil
}

// newFCMSender membuat pengirim FCM dari konfigurasi. Kredensial kosong atau rusak hanya dicatat sebagai
// peringatan dan mengembalikan nil: notify-service tetap jalan untuk Web Push.
func newFCMSender(cfg config.Config) *fcm.Sender {
	// FCM tidak dikonfigurasi: matikan tanpa peringatan bila project juga kosong
	if cfg.FCMCredentialsFile == "" {
		if cfg.FCMProjectID != "" {
			slog.Warn("FCM_PROJECT_ID set but FCM_CREDENTIALS_FILE empty; Android push disabled")
		}
		return nil
	}
	// Baca berkas JSON service account (di container: /run/fcm/..., di-mount dari keys/fcm/). Galat paling umum di
	// sini: path salah, atau izin berkas tidak memberi akses ke UID 10001 (user container).
	data, err := os.ReadFile(cfg.FCMCredentialsFile)
	if err != nil {
		slog.Warn("read FCM credentials failed; Android push disabled", "err", err)
		return nil
	}
	// Ubah JSON ke Credentials; gagal bila berkas bukan JSON service account (mis. salah unduh google-services.json).
	creds, err := fcm.ParseCredentials(data)
	if err != nil {
		slog.Warn("parse FCM credentials failed; Android push disabled", "err", err)
		return nil
	}
	// Siapkan pengirim; kunci privat diperiksa di sini sehingga kredensial rusak langsung terlihat saat start.
	sender, err := fcm.New(cfg.FCMProjectID, creds)
	if err != nil {
		slog.Warn("init FCM sender failed; Android push disabled", "err", err)
		return nil
	}
	// Penanda di log bahwa push ke aplikasi Android aktif (dipakai saat verifikasi deploy).
	slog.Info("FCM push enabled")
	return sender
}
