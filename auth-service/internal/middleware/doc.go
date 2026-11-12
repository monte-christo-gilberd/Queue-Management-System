// Package middleware berisi handler middleware Gin yang dipasang di router sebelum handler endpoint.
//
// Middleware dibuat di cmd/auth-service/main.go (Auth) lalu diberikan ke router.NewAppRouter. RequestID, Logger, dan
// Recovery dipasang langsung oleh router.SetupRouter.
//
// Isinya:
//   - request_id.go: RequestID, memberi setiap request id (dari X-Request-Id nginx, atau dibuat baru) untuk korelasi log.
//   - logger.go: Logger, mencatat satu baris JSON per request (templat route, tanpa query string, user_id angka saja).
//   - recovery.go: Recovery, menangkap panic, mencatatnya lewat slog, dan membalas 500 dengan body error yang seragam.
//   - auth.go: Auth, yang memeriksa JWT RS256 lewat interface Authenticator (token tidak sah atau sesi dicabut: 401),
//     lalu menyimpan id user di context dengan kunci "user_id".
//   - abort_response.go: abortError, pembantu untuk menghentikan request dengan body JSON error yang seragam.
package middleware
