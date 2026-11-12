package metrics

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// PendingFunc menghitung baris outbox yang belum terkirim dan belum gagal permanen.
type PendingFunc func(ctx context.Context) (int64, error)

// Mail memegang metrik bisnis mail-service.
type Mail struct {
	*Metrics
	sent   prometheus.Counter
	failed prometheus.Counter
}

// NewMail membuat Metrics mail-service. Gauge mail_outbox_pending dipasang lewat SetPending setelah database tersambung.
func NewMail() *Mail {
	m := &Mail{Metrics: New("mail-service")}
	m.sent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mail_sent_total", Help: "Email outbox yang berhasil dikirim (ditandai terkirim).",
	})
	m.failed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mail_failed_total", Help: "Email outbox yang gagal permanen (batas percobaan habis atau galat permanen).",
	})
	m.Reg.MustRegister(m.sent, m.failed)
	return m
}

// Middleware mengembalikan middleware HTTP umum (tanpa hook bisnis; metrik email ada di decorator outbox).
func (m *Mail) Middleware() gin.HandlerFunc { return m.HTTPMiddleware(nil) }

// IncSent menambah mail_sent_total. Dipanggil decorator store outbox saat MarkSent berhasil.
func (m *Mail) IncSent() { m.sent.Inc() }

// IncFailed menambah mail_failed_total. Dipanggil decorator store outbox saat MarkFailed berhasil.
func (m *Mail) IncFailed() { m.failed.Inc() }

// SetPending memasang gauge mail_outbox_pending yang dihitung setiap scrape lewat fn (dengan batas 2 detik).
// Bila query gagal, sampel dilewati (tidak diganti angka palsu).
func (m *Mail) SetPending(fn PendingFunc) {
	m.Reg.MustRegister(&pendingCollector{fn: fn})
}

// pendingDesc: deskripsi gauge mail_outbox_pending.
var pendingDesc = prometheus.NewDesc("mail_outbox_pending", "Baris outbox email yang belum terkirim dan belum gagal permanen (antrean email menumpuk bila naik terus).", nil, nil)

// pendingCollector menghitung antrean outbox saat Prometheus scrape.
type pendingCollector struct{ fn PendingFunc }

// Describe mengumumkan metrik yang dihasilkan.
func (p *pendingCollector) Describe(ch chan<- *prometheus.Desc) { ch <- pendingDesc }

// Collect menjalankan query hitung; galat membuat sampel dilewati.
func (p *pendingCollector) Collect(ch chan<- prometheus.Metric) {
	// batas waktu agar database lambat tidak menahan scrape
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	n, err := p.fn(ctx)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(pendingDesc, err)
		return
	}
	ch <- prometheus.MustNewConstMetric(pendingDesc, prometheus.GaugeValue, float64(n))
}
