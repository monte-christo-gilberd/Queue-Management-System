// Package middleware berisi handler middleware Gin yang dipasang di router sebelum handler endpoint.
//
// Middleware dipasang di cmd/server/main.go (RequireAuthWithRevocation) dan internal/router/router.go (RequestID,
// Logger, Recovery, dan CORS).
//
// Isinya:
//   - auth.go: RequireAuth dan RequireAuthWithRevocation untuk memeriksa JWT, serta interface SessionRevocationChecker
//     untuk pencabutan sesi (token yang terbit sebelum password diganti ditolak).
//   - request_id.go: RequestID, memberi setiap request id (dari X-Request-Id nginx, atau dibuat baru) untuk korelasi log.
//   - logger.go: Logger, mencatat satu baris JSON per request (templat route, tanpa query string, user_id angka saja).
//   - recovery.go: Recovery, menangkap panic, mencatatnya lewat slog, dan membalas 500 dengan body error yang seragam.
//   - cors.go: CORS, yang hanya memberi izin ke origin yang terdaftar.
//   - abort_response.go: abortError, pembantu untuk menghentikan request dengan body JSON error yang seragam.
package middleware
