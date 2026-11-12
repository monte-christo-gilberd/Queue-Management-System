// Package middleware berisi middleware gin yang dipasang di router sebelum handler endpoint.
//
// Middleware dipasang di cmd/mail-service/main.go dan dipakai oleh internal/router.
//
// Isinya:
//   - request_id.go: RequestID, memberi setiap request id (dari X-Request-Id, atau dibuat baru) untuk korelasi log.
//   - logger.go: Logger, mencatat satu baris JSON per request (templat route, tanpa query string).
//   - recovery.go: Recovery, menangkap panic, mencatatnya lewat slog, dan membalas 500 dengan body error yang seragam.
//   - service_token.go: ServiceToken, yang memeriksa header X-Service-Token dengan perbandingan waktu tetap dan
//     menjawab 401 UNAUTHORIZED (body error JSON yang seragam dengan handler) bila tidak cocok.
package middleware
