// Package metrics menyediakan metrik Prometheus qr-service: registry berlabel service, listener /metrics terpisah
// (:9100, tidak dipublikasikan, tidak pernah menggagalkan service), dan metrik bisnisnya.
//
// Isinya:
//   - metrics.go: New, Handler, Serve (listener terpisah yang tidak pernah menggagalkan service).
//   - http.go: HTTPMiddleware, metrik request umum dengan label route berupa templat.
//   - qr.go: Metrics qr-service (HTTP umum, tanpa metrik bisnis).
package metrics
