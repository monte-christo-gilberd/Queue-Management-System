// Package middleware berisi middleware Gin yang dipasang di router sebelum handler endpoint.
//
// Middleware global (semua rute) dipasang di internal/router/router.go: RequestID, Logger, dan NoStore.
// Middleware grup internal (/internal/v1): BodyLimit lalu ServiceToken.
//
// Isinya:
//   - abort_response.go: abortWithError, pembantu untuk menghentikan request dengan body JSON error yang seragam.
//   - service_token.go: ServiceToken, memeriksa header X-Service-Token dengan perbandingan constant time.
//   - body_limit.go: BodyLimit, menolak body yang lebih besar dari batas.
//   - request_id.go: RequestID, memberi setiap request id untuk korelasi log.
//   - logger.go: Logger, mencatat satu baris JSON per request (hanya path, tanpa query string).
//   - no_store.go: NoStore, melarang cache untuk respons yang berisi token atau data sensitif.
package middleware
