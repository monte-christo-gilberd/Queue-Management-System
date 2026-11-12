package metrics

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// Templat route yang dipakai hook bisnis. Harus sama dengan yang didaftarkan di internal/router (dijaga tes).
const (
	routeJoin      = "/api/v1/join"
	routeQueues    = "/api/v1/queues"
	routeInsert    = "/api/v1/queues/:queue_id/insert"
	routeNext      = "/api/v1/queues/:queue_id/next"
	routeSubExtend = "/api/v1/owners/subscription/extend"
)

// ClientCounter memberi jumlah koneksi SSE aktif: queue untuk room antrian, display untuk layar publik.
// Dipenuhi *sse.Hub.
type ClientCounter interface {
	ClientCounts() (queue int, display int)
}

// Backend memegang metrik bisnis backend.
type Backend struct {
	*Metrics
	// ticketsJoined: tiket baru masuk antrian (join lewat QR/token atau disisipkan owner).
	ticketsJoined prometheus.Counter
	// ticketsServed: panggilan "Next" yang berhasil (tiket mulai dilayani).
	ticketsServed prometheus.Counter
	// subInactive: penolakan 402 SUBSCRIPTION_INACTIVE, per route.
	subInactive *prometheus.CounterVec
	// subExtend: hasil tambah langganan (ok, email_not_verified, error).
	subExtend *prometheus.CounterVec
	// emailRejected: penolakan EMAIL_NOT_VERIFIED, per aksi (queue_create, join, subscription_extend).
	emailRejected *prometheus.CounterVec
}

// NewBackend membuat Metrics backend lengkap dengan metrik bisnis. clients boleh nil (gauge SSE tidak dipasang).
func NewBackend(
	clients ClientCounter,
) *Backend {
	// registry dasar dengan runtime Go dan proses
	b := &Backend{Metrics: New("backend")}
	// penghitung tiket
	b.ticketsJoined = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tickets_joined_total",
		Help: "Tiket yang masuk antrian (join lewat token atau disisipkan owner).",
	})
	b.ticketsServed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tickets_served_total",
		Help: "Panggilan tiket berikutnya (Next) yang berhasil, yaitu tiket mulai dilayani.",
	})
	// penolakan langganan tidak aktif
	b.subInactive = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "subscription_inactive_rejected_total",
		Help: "Request yang ditolak 402 SUBSCRIPTION_INACTIVE, per templat route.",
	}, []string{"route"})
	// hasil perpanjangan langganan
	b.subExtend = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "subscription_extend_total",
		Help: "Hasil POST /owners/subscription/extend: ok, email_not_verified, atau error.",
	}, []string{"result"})
	// penolakan karena email belum diverifikasi
	b.emailRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "email_not_verified_rejected_total",
		Help: "Request yang ditolak EMAIL_NOT_VERIFIED, per aksi: queue_create, join, subscription_extend.",
	}, []string{"action"})
	b.Reg.MustRegister(b.ticketsJoined, b.ticketsServed, b.subInactive, b.subExtend, b.emailRejected)
	// nilai awal 0 untuk label yang diketahui, supaya grafik dan alert tidak menunggu kejadian pertama
	for _, r := range []string{"ok", "email_not_verified", "error"} {
		b.subExtend.WithLabelValues(r)
	}
	for _, a := range []string{"queue_create", "join", "subscription_extend"} {
		b.emailRejected.WithLabelValues(a)
	}
	// gauge koneksi SSE aktif dibaca dari hub saat di-scrape
	if clients != nil {
		b.Reg.MustRegister(&sseCollector{clients: clients})
	}
	return b
}

// Middleware mengembalikan middleware HTTP umum yang juga memicu hook metrik bisnis.
func (b *Backend) Middleware() gin.HandlerFunc {
	return b.HTTPMiddleware(b.hook)
}

// hook membaca hasil request untuk metrik bisnis. Tidak mengubah logika bisnis: hanya melihat route, status, dan
// kode galat yang dicatat respondError di context.
func (b *Backend) hook(
	c *gin.Context,
	route string,
	status int,
) {
	// kode galat aplikasi (kosong bila respons sukses)
	code := c.GetString("error_code")
	method := c.Request.Method
	ok := status >= 200 && status < 300

	switch {
	// tiket baru masuk: join lewat token atau disisipkan owner
	case method == http.MethodPost && (route == routeJoin || route == routeInsert) && ok:
		b.ticketsJoined.Inc()
	// tiket mulai dilayani: Next berhasil
	case method == http.MethodPost && route == routeNext && ok:
		b.ticketsServed.Inc()
	}
	// langganan tidak aktif ditolak di route mana pun
	if code == "SUBSCRIPTION_INACTIVE" {
		b.subInactive.WithLabelValues(route).Inc()
	}
	// hasil perpanjangan langganan
	if method == http.MethodPost && route == routeSubExtend {
		switch {
		case ok:
			b.subExtend.WithLabelValues("ok").Inc()
		case code == "EMAIL_NOT_VERIFIED":
			b.subExtend.WithLabelValues("email_not_verified").Inc()
		default:
			b.subExtend.WithLabelValues("error").Inc()
		}
	}
	// penolakan karena email belum diverifikasi, per aksi
	if code == "EMAIL_NOT_VERIFIED" && method == http.MethodPost {
		switch route {
		case routeQueues:
			b.emailRejected.WithLabelValues("queue_create").Inc()
		case routeJoin:
			b.emailRejected.WithLabelValues("join").Inc()
		case routeSubExtend:
			b.emailRejected.WithLabelValues("subscription_extend").Inc()
		}
	}
}

// sseCollector membaca jumlah koneksi SSE aktif dari hub setiap kali Prometheus scrape.
type sseCollector struct {
	clients ClientCounter
}

// desc: deskripsi gauge sse_clients_active dengan label room (queue atau display).
var sseDesc = prometheus.NewDesc("sse_clients_active", "Koneksi SSE aktif, per jenis room: queue (antrian) atau display (layar publik).", []string{"room"}, nil)

// Describe mengumumkan metrik yang dihasilkan collector ini.
func (s *sseCollector) Describe(ch chan<- *prometheus.Desc) { ch <- sseDesc }

// Collect membaca jumlah klien saat ini dan mengirimnya sebagai dua gauge.
func (s *sseCollector) Collect(ch chan<- prometheus.Metric) {
	// jumlah klien di room antrian dan layar publik
	q, d := s.clients.ClientCounts()
	ch <- prometheus.MustNewConstMetric(sseDesc, prometheus.GaugeValue, float64(q), "queue")
	ch <- prometheus.MustNewConstMetric(sseDesc, prometheus.GaugeValue, float64(d), "display")
}
