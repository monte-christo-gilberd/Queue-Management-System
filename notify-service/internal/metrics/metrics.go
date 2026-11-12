package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// defaultAddr: alamat listener metrik bila METRICS_ADDR kosong. Port terpisah dari API, tidak dipublikasikan
// ke host, dan tidak diteruskan nginx; hanya Prometheus di jaringan internal yang mengambilnya.
const defaultAddr = ":9100"

// Metrics menyimpan registry Prometheus satu service. Semua metrik yang didaftarkan lewat Reg otomatis membawa
// label service=<nama service>.
type Metrics struct {
	// Registry: registry khusus service ini (bukan registry global) supaya tes tidak saling bentrok.
	Registry *prometheus.Registry
	// Reg: pendaftar yang menambahkan label service ke setiap metrik. Pakai ini untuk mendaftarkan metrik baru.
	Reg prometheus.Registerer
	// service: nama service, dipakai sebagai nilai label service.
	service string
}

// New membuat Metrics untuk service dan mendaftarkan metrik runtime Go dan proses.
func New(
	service string,
) *Metrics {
	// registry baru, bukan DefaultRegisterer, agar hanya metrik yang kita daftarkan yang muncul
	reg := prometheus.NewRegistry()
	// pembungkus yang menempelkan label service ke semua metrik di bawahnya
	wrapped := prometheus.WrapRegistererWith(prometheus.Labels{"service": service}, reg)
	// metrik runtime Go (goroutine, GC, memori) dan proses (CPU, file descriptor)
	wrapped.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	// kembalikan registry dan pendaftar berlabel
	return &Metrics{Registry: reg, Reg: wrapped, service: service}
}

// Handler mengembalikan http.Handler yang menyajikan isi registry dalam format teks Prometheus.
func (m *Metrics) Handler() http.Handler {
	// ErrorHandling Continue: metrik yang gagal dikumpulkan dilewati, sisanya tetap disajikan
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{ErrorHandling: promhttp.ContinueOnError})
}

// listenAddr membaca METRICS_ADDR. Kosong memakai :9100, "off" mematikan listener (mengembalikan "").
func listenAddr() string {
	// nilai dari environment, tanpa spasi di pinggir
	v := strings.TrimSpace(os.Getenv("METRICS_ADDR"))
	// kosong: pakai alamat bawaan
	if v == "" {
		return defaultAddr
	}
	// "off": operator sengaja mematikan listener metrik
	if strings.EqualFold(v, "off") {
		return ""
	}
	// alamat khusus (mis. :9101 saat beberapa service jalan di satu host)
	return v
}

// Serve menjalankan listener metrik (GET /metrics) di goroutine sendiri sampai ctx batal. Tidak memblokir dan tidak
// pernah menggagalkan service: bila port tidak bisa dipakai (mis. bentrok saat beberapa service jalan di satu host
// tanpa Docker), hanya peringatan yang dicatat dan aplikasi berjalan terus tanpa metrik.
func (m *Metrics) Serve(
	ctx context.Context,
) {
	// alamat dari env; kosong berarti dimatikan
	addr := listenAddr()
	if addr == "" {
		slog.Info("metrics listener disabled (METRICS_ADDR=off)")
		return
	}
	// mux terpisah dari API: hanya /metrics, tidak ada rute lain
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())
	// timeout dasar agar koneksi macet tidak menggantung
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	// hentikan listener saat service berhenti
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	// jalankan di goroutine; kegagalan hanya dicatat
	go func() {
		slog.Info("metrics listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Warn("metrics listener stopped; service continues without metrics", "addr", addr, "err", err)
		}
	}()
}
