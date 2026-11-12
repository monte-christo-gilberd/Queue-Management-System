// Package metrics menyediakan metrik Prometheus backend: registry berlabel service, middleware HTTP, listener
// /metrics terpisah (:9100, tidak dipublikasikan), dan metrik bisnis (antrian, SSE, langganan).
//
// Isinya:
//   - metrics.go: New, Handler, Serve (listener terpisah yang tidak pernah menggagalkan service).
//   - http.go: HTTPMiddleware, metrik request umum dengan label route berupa templat.
//   - backend.go: metrik bisnis backend dan hook yang membacanya dari hasil request.
package metrics
