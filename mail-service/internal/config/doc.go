// Package config membaca dan memvalidasi konfigurasi mail-service dari environment variable.
//
// Load() adalah pintu masuk utama: memeriksa token layanan, SMTP (LoadSMTP), dan pengaturan outbox (loadOutbox).
// Konfigurasi yang tidak lengkap atau tidak masuk akal ditolak saat start, supaya email tidak diam-diam gagal terkirim.
//
// Isinya:
//   - config.go: Config, Outbox, dan Load.
//   - smtp.go: SMTP, Addr, dan LoadSMTP. LoadSMTP juga dipakai oleh perintah send-test di cmd/mail-service.
//
// Package ini tidak membuka koneksi apa pun, ia hanya membaca nilai.
package config
