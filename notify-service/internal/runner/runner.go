package runner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"qms/notify-service/internal/model"
)

const (
	// eventTimeout: batas waktu pemrosesan satu event.
	eventTimeout = 20 * time.Second
	// reconnectDelay: jeda sebelum menyambung ulang listener setelah koneksi putus.
	reconnectDelay = 3 * time.Second
	// drainTimeout: batas menunggu proses kirim yang masih berjalan saat shutdown.
	drainTimeout = 10 * time.Second
)

// EventHandler: pemroses satu event. Dipenuhi service.NotifyService.
type EventHandler interface {
	// Handle: memproses satu event. Galat dicatat oleh Runner dan tidak menghentikan listener.
	Handle(
		ctx context.Context,
		e model.Event,
	) error
}

// ListenFunc: fungsi yang mendengarkan event sampai koneksi putus atau ctx selesai. Dipenuhi listener.Listen
// setelah DSN dipasang.
type ListenFunc func(
	ctx context.Context,
	handle func(model.Event),
	onState func(bool),
) error

// Runner: menjalankan listener dengan sambung ulang, dan memproses tiap event di goroutine sendiri.
type Runner struct {
	// handler: pemroses event.
	handler EventHandler
	// listen: fungsi pendengar event.
	listen ListenFunc
	// onState: callback status LISTEN untuk health. Boleh nil.
	onState func(bool)
	// inflight: hitungan proses kirim yang masih berjalan, supaya shutdown bisa menunggunya.
	inflight sync.WaitGroup
}

// New membuat Runner dengan handler, fungsi listen, dan callback status LISTEN (boleh nil).
func New(
	handler EventHandler,
	listen ListenFunc,
	onState func(bool),
) *Runner {
	// Simpan semua dependensi ke Runner.
	return &Runner{handler: handler, listen: listen, onState: onState}
}

// setState meneruskan status LISTEN ke callback bila ada.
func (r *Runner) setState(
	up bool,
) {
	// Callback dipasang: teruskan statusnya.
	if r.onState != nil {
		r.onState(up)
	}
}

// dispatch menjalankan satu event di goroutine sendiri agar listener tidak tertahan oleh pengiriman.
func (r *Runner) dispatch(
	e model.Event,
) {
	// Tandai satu proses kirim yang sedang berjalan.
	r.inflight.Add(1)
	// Proses di goroutine sendiri supaya listener langsung kembali menunggu notifikasi.
	go func() {
		// Tandai selesai saat goroutine berakhir.
		defer r.inflight.Done()
		// Beri setiap event batas waktu 20 detik.
		sendCtx, cancel := context.WithTimeout(context.Background(), eventTimeout)
		defer cancel()
		// Proses event. Galat hanya dicatat, tidak menghentikan listener.
		if err := r.handler.Handle(sendCtx, e); err != nil {
			slog.Error(
				"handle event",
				"kind",
				e.Kind,
				"ticket_id",
				e.TicketID,
				"err",
				err,
			)
			return
		}
		// Event selesai diproses: catat jenis dan id tiket (angka saja, tanpa isi notifikasi atau data pengguna).
		slog.Info(
			"event handled",
			"kind",
			e.Kind,
			"ticket_id",
			e.TicketID,
		)
	}()
}

// Run mendengarkan event sampai ctx batal, lalu menunggu proses kirim yang masih berjalan (maksimal 10 detik).
// Koneksi putus: sambung ulang setelah 3 detik. Sinyal selama putus hilang (outbox menyusul).
func (r *Runner) Run(
	ctx context.Context,
) {
	// Ulangi selama konteks belum batal.
	for ctx.Err() == nil {
		// Jalankan listener sampai koneksi putus. Status listening diperbarui lewat callback.
		err := r.listen(ctx, r.dispatch, r.setState)
		// Listener berhenti: health harus menunjukkan tidak tersambung.
		r.setState(false)
		// Berhenti karena galat (bukan karena shutdown): tunggu 3 detik lalu sambung ulang.
		if err != nil && ctx.Err() == nil {
			slog.Warn("listener stopped, reconnecting in 3s", "err", err)
			// Tunggu 3 detik, atau berhenti lebih cepat bila shutdown dimulai.
			select {
			// Shutdown dimulai: keluar tanpa menunggu jeda.
			case <-ctx.Done():
			// Jeda selesai: loop luar menyambung ulang. Jeda mencegah putaran cepat saat database mati.
			case <-time.After(reconnectDelay):
			}
		}
	}

	// Konteks batal: mulai shutdown. Proses kirim yang masih berjalan ditunggu supaya push yang sudah
	// diterima tidak hilang di tengah jalan.
	slog.Info("shutting down, waiting for in-flight pushes")
	// Channel yang ditutup setelah semua proses kirim selesai.
	done := make(chan struct{})
	// Goroutine pembantu: menunggu semua proses kirim, lalu menutup done.
	go func() { r.inflight.Wait(); close(done) }()
	// Tunggu sampai semua selesai, atau berhenti setelah 10 detik.
	select {
	// Semua proses kirim selesai.
	case <-done:
	// Waktu habis: catat bahwa proses kirim yang belum selesai dibatalkan.
	case <-time.After(drainTimeout):
		slog.Warn("shutdown timeout, dropping in-flight pushes")
	}
}
