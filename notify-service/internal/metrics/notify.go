package metrics

import "github.com/prometheus/client_golang/prometheus"

// channels: kanal push yang dihitung terpisah (label channel).
var channels = []string{"webpush", "fcm"}

// Notify memegang metrik bisnis notify-service.
type Notify struct {
	*Metrics
	// sent: jumlah push yang berhasil dikirim, per kanal (webpush atau fcm).
	sent *prometheus.CounterVec
	// failed: jumlah push yang gagal, per kanal dan alasan (gone atau error).
	failed *prometheus.CounterVec
}

// NewNotify membuat Metrics notify-service. Service ini tidak punya API HTTP selain /health, jadi tidak ada metrik request.
func NewNotify() *Notify {
	// registry dasar dengan runtime Go dan proses
	n := &Notify{Metrics: New("notify-service")}
	// penghitung push terkirim per kanal
	n.sent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "push_sent_total", Help: "Push yang berhasil dikirim, per kanal (webpush = browser, fcm = aplikasi Android).",
	}, []string{"channel"})
	// penghitung push gagal, dipisah menurut kanal dan alasan supaya "gone" yang wajar tidak tercampur dengan galat sungguhan
	n.failed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "push_failed_total", Help: "Push yang gagal per kanal: gone (langganan atau token sudah tidak berlaku) atau error (galat lain).",
	}, []string{"channel", "reason"})
	n.Reg.MustRegister(n.sent, n.failed)
	// nilai awal 0 untuk semua kombinasi, supaya grafik dan alert tidak menunggu kejadian pertama
	for _, c := range channels {
		n.sent.WithLabelValues(c)
		n.failed.WithLabelValues(c, "gone")
		n.failed.WithLabelValues(c, "error")
	}
	return n
}

// IncSent menambah push_sent_total untuk kanal channel. Dipanggil pembungkus pengirim saat Send berhasil.
func (n *Notify) IncSent(channel string) { n.sent.WithLabelValues(channel).Inc() }

// IncFailed menambah push_failed_total untuk kanal channel dengan alasan gone atau error.
func (n *Notify) IncFailed(channel, reason string) { n.failed.WithLabelValues(channel, reason).Inc() }
