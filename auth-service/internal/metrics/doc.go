// Package metrics menyediakan metrik Prometheus auth-service: registry berlabel service, listener /metrics terpisah
// (:9100, tidak dipublikasikan, tidak pernah menggagalkan service), dan metrik bisnisnya.
//
// Isinya:
//   - metrics.go: New, Handler, Serve (listener terpisah yang tidak pernah menggagalkan service).
//   - http.go: HTTPMiddleware, metrik request umum dengan label route berupa templat.
//   - auth.go: metrik login, registrasi, reset password, dan verifikasi email, dibaca dari hasil request.
package metrics
