// Package middleware berisi handler middleware Gin yang dipasang di router sebelum handler endpoint.
//
// Middleware dipasang di cmd/server/main.go (RequireAuthWithRevocation) dan internal/router/router.go (CORS).
//
// Isinya:
//   - auth.go: RequireAuth dan RequireAuthWithRevocation untuk memeriksa JWT, serta interface SessionRevocationChecker
//     untuk pencabutan sesi (token yang terbit sebelum password diganti ditolak).
//   - cors.go: CORS, yang hanya memberi izin ke origin yang terdaftar.
//   - abort_response.go: abortError, pembantu untuk menghentikan request dengan body JSON error yang seragam.
package middleware
