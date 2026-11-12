package metrics

import "github.com/prometheus/client_golang/prometheus"

// Notify memegang metrik bisnis notify-service.
type Notify struct {
	*Metrics
	// sent: jumlah push yang berhasil dikirim ke push service.
	sent prometheus.Counter
	// failed: jumlah push yang gagal, per alasan (gone atau error).
	failed *prometheus.CounterVec
}

// NewNotify membuat Metrics notify-service. Service ini tidak punya API HTTP selain /health, jadi tidak ada metrik request.
func NewNotify() *Notify {
	// registry dasar dengan runtime Go dan proses
	n := &Notify{Metrics: New("notify-service")}
	// penghitung push terkirim
	n.sent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "push_sent_total", Help: "Web Push yang berhasil dikirim ke push service.",
	})
	// penghitung push gagal, dipisah menurut alasan supaya "gone" yang wajar tidak tercampur dengan galat sungguhan
	n.failed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "push_failed_total", Help: "Web Push yang gagal: gone (langganan sudah tidak berlaku, 404/410) atau error (galat lain).",
	}, []string{"reason"})
	n.Reg.MustRegister(n.sent, n.failed)
	// nilai awal 0 untuk kedua alasan, supaya grafik dan alert tidak menunggu kejadian pertama
	n.failed.WithLabelValues("gone")
	n.failed.WithLabelValues("error")
	return n
}

// IncSent menambah push_sent_total. Dipanggil pembungkus pengirim saat Send berhasil.
func (n *Notify) IncSent() { n.sent.Inc() }

// IncFailed menambah push_failed_total dengan alasan gone atau error. Dipanggil pembungkus pengirim saat Send gagal.
func (n *Notify) IncFailed(reason string) { n.failed.WithLabelValues(reason).Inc() }
