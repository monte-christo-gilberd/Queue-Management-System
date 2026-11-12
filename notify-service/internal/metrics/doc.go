// Package metrics menyediakan metrik Prometheus notify-service: registry berlabel service, listener /metrics terpisah
// (:9100, tidak dipublikasikan, tidak pernah menggagalkan service), dan metrik bisnisnya.
//
// Isinya:
//   - metrics.go: New, Handler, Serve (listener terpisah yang tidak pernah menggagalkan service).
//   - notify.go: push_sent_total dan push_failed_total.
package metrics
