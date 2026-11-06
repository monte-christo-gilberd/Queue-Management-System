// Package config membaca dan memvalidasi konfigurasi backend dari environment variable.
//
// Satu-satunya pintu masuk adalah Load(), yang dipanggil di cmd/server/main.go. Load() mengisi Config,
// menerapkan nilai bawaan (PORT, QR_HTTP_TIMEOUT), lalu menolak konfigurasi yang kurang atau tidak aman.
// Pengetatan tambahan untuk APP_ENV=production ada di validateProduction().
//
// Package ini tidak membuka koneksi apa pun, ia hanya membaca nilai.
package config
