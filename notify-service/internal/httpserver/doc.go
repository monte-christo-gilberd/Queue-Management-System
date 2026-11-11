// Package httpserver berisi server HTTP kecil untuk health check notify-service.
//
// Isinya:
//   - httpserver.go: Health (status LISTEN, SetListening, Handler) dan New (server dengan batas waktu header 5 detik).
//
// GET /health membalas 200 {"status":"ok"} hanya selama LISTEN queue_events tersambung, dan 503 "not listening"
// selama belum atau sudah putus. Package ini tidak ada endpoint lain.
package httpserver
