// Package requestid menyimpan dan membaca request ID di context.Context.
//
// Middleware RequestID mengisinya di awal request; qrclient membacanya untuk meneruskan header X-Request-Id ke
// qr-service, sehingga satu request bisa diikuti di log kedua service.
//
// Isinya:
//   - requestid.go: With dan From.
package requestid
