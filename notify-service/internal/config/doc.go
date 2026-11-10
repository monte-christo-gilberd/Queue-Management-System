// Package config membaca dan memvalidasi konfigurasi notify-service dari environment variable.
//
// Satu-satunya pintu masuk adalah Load(), yang dipanggil di cmd/notify-service/main.go. Load() gagal cepat
// bila ada variabel wajib yang kosong, sehingga container tidak jalan dengan konfigurasi setengah.
//
// Package ini tidak membuka koneksi apa pun, ia hanya membaca nilai.
package config
