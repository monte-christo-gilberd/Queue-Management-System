// Package middleware berisi pembungkus handler net/http yang dipasang di router sebelum handler endpoint.
//
// Middleware dipasang di cmd/mail-service/main.go dan dipakai oleh internal/router.
//
// Isinya:
//   - service_token.go: ServiceToken, yang memeriksa header X-Service-Token dengan perbandingan waktu tetap dan
//     menjawab 401 UNAUTHORIZED bila tidak cocok. abortError membentuk body error JSON yang seragam dengan handler.
package middleware
