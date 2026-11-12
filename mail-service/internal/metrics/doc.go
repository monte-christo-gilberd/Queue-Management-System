// Package metrics menyediakan metrik Prometheus mail-service: registry berlabel service, listener /metrics terpisah
// (:9100, tidak dipublikasikan, tidak pernah menggagalkan service), dan metrik bisnisnya.
//
// Isinya:
//   - metrics.go: New, Handler, Serve (listener terpisah yang tidak pernah menggagalkan service).
//   - http.go: HTTPMiddleware, metrik request umum dengan label route berupa templat.
//   - mail.go: mail_outbox_pending, mail_sent_total, mail_failed_total.
package metrics
