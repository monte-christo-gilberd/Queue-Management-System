package metrics

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// Hook dipanggil setelah tiap request selesai dengan templat route, status, dan konteks (mis. kode galat aplikasi
// di c.GetString("error_code")). Dipakai service untuk metrik bisnis yang bisa dibaca dari hasil request,
// tanpa mengubah handler bisnisnya.
type Hook func(c *gin.Context, route string, status int)

// HTTPMiddleware mencatat metrik HTTP umum: http_requests_total{route,method,status},
// http_request_duration_seconds{route,method}, dan http_requests_in_flight. Label route memakai TEMPLAT
// (c.FullPath()), bukan path mentah, supaya jumlah nilai label kecil dan token di URL tidak ikut tercatat.
// Rute SSE (berakhiran /event) tidak dihitung di durasi dan in-flight karena koneksinya berjam-jam dan merusak
// persentil; permintaannya tetap dihitung di http_requests_total. Panggil sekali per Metrics (mendaftarkan metrik).
func (m *Metrics) HTTPMiddleware(
	hook Hook,
) gin.HandlerFunc {
	// penghitung request per route, method, dan status
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Jumlah request HTTP selesai, per templat route, method, dan status.",
	}, []string{"route", "method", "status"})
	// histogram durasi per route dan method; bucket dari 5 ms sampai 10 detik. Dari histogram inilah p95 dihitung.
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Durasi request HTTP dalam detik, per templat route dan method (tanpa rute SSE).",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"route", "method"})
	// jumlah request yang sedang diproses; naik terus berarti request menumpuk
	inFlight := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Jumlah request HTTP yang sedang diproses (tanpa rute SSE).",
	})
	// daftarkan ketiganya; label service ditempel otomatis oleh Reg
	m.Reg.MustRegister(requests, duration, inFlight)

	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// templat rute; kosong berarti tidak ada rute (404), jadi pakai penanda tetap
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		// koneksi SSE berumur panjang: dikecualikan dari in-flight dan durasi
		sse := strings.HasSuffix(route, "/event")
		// waktu mulai untuk menghitung durasi
		start := time.Now()
		// request mulai diproses
		if !sse {
			inFlight.Inc()
		}
		// jalankan handler dan middleware berikutnya
		c.Next()
		// status akhir respons (setelah Recovery, bila ada panic yang ditangkap)
		status := c.Writer.Status()
		method := c.Request.Method
		// hitung request selesai; status diubah ke teks karena label Prometheus selalu string
		requests.WithLabelValues(route, method, strconv.Itoa(status)).Inc()
		if !sse {
			// request selesai diproses, lalu catat durasinya
			inFlight.Dec()
			duration.WithLabelValues(route, method).Observe(time.Since(start).Seconds())
		}
		// metrik bisnis khusus service, bila ada
		if hook != nil {
			hook(c, route, status)
		}
	}
}
