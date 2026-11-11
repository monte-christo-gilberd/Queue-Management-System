// Package config membaca dan memvalidasi konfigurasi qr-service dari environment variable.
//
// Satu-satunya pintu masuk adalah Load(), yang dipanggil di cmd/qr-service/main.go (run). Load() mengisi Config,
// menerapkan nilai bawaan (PORT, LOG_LEVEL, QR_DYNAMIC_TTL_SECONDS), lalu menolak konfigurasi yang kosong
// atau tidak aman (secret terlalu pendek, token internal tidak diisi, TTL di luar rentang).
//
// Package ini tidak membuka koneksi apa pun, ia hanya membaca nilai.
package config
