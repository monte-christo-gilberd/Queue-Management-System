package metrics

import "github.com/prometheus/client_golang/prometheus"

// Notify memegang metrik bisnis notify-service.
type Notify struct {
	*Metrics
	sent   prometheus.Counter
	failed *prometheus.CounterVec
}

// NewNotify membuat Metrics notify-service. Service ini tidak punya API HTTP selain /health, jadi tidak ada metrik request.
func NewNotify() *Notify {
	n := &Notify{Metrics: New("notify-service")}
	n.sent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "push_sent_total", Help: "Web Push yang berhasil dikirim ke push service.",
	})
	n.failed = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "push_failed_total", Help: "Web Push yang gagal: gone (langganan sudah tidak berlaku, 404/410) atau error (galat lain).",
	}, []string{"reason"})
	n.Reg.MustRegister(n.sent, n.failed)
	n.failed.WithLabelValues("gone")
	n.failed.WithLabelValues("error")
	return n
}

// IncSent menambah push_sent_total.
func (n *Notify) IncSent() { n.sent.Inc() }

// IncFailed menambah push_failed_total dengan alasan gone atau error.
func (n *Notify) IncFailed(reason string) { n.failed.WithLabelValues(reason).Inc() }
