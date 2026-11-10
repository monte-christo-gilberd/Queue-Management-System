package config

import (
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"strings"
)

// SMTP: pengaturan pengiriman email. TLS: "none" (Mailpit di dev), "starttls" (port 587), atau "tls" (port 465).
type SMTP struct {
	// Host: nama host server SMTP (SMTP_HOST).
	Host string
	// Port: port server SMTP (SMTP_PORT), 1 sampai 65535.
	Port int
	// From: alamat pengirim, mis. "QMS <no-reply@contoh.id>" (SMTP_FROM).
	From string
	// User: nama pengguna autentikasi SMTP (SMTP_USER). Kosong bila server tidak butuh autentikasi.
	User string
	// Password: kata sandi autentikasi SMTP (SMTP_PASSWORD). Wajib diisi bersama User.
	Password string
	// TLS: mode enkripsi: none, starttls, atau tls (SMTP_TLS). Bawaan none.
	TLS string
}

// Addr: host:port untuk dial.
func (s SMTP) Addr() string { return fmt.Sprintf("%s:%d", s.Host, s.Port) }

// LoadSMTP membaca SMTP_* dan gagal cepat bila tidak lengkap atau tidak masuk akal, sama seperti VAPID:
// lebih baik service menolak start daripada email diam-diam tidak pernah terkirim.
func LoadSMTP() (SMTP, error) {
	// Baca variabel SMTP dari environment, dengan spasi di tepi dibuang.
	s := SMTP{
		Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		From:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		TLS:      strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_TLS"))),
	}
	// Host kosong: tidak bisa mengirim, tolak.
	if s.Host == "" {
		return SMTP{}, fmt.Errorf("missing required env var: SMTP_HOST")
	}
	// Alamat pengirim kosong: tolak.
	if s.From == "" {
		return SMTP{}, fmt.Errorf("missing required env var: SMTP_FROM")
	}
	// Alamat pengirim tidak bisa diurai sebagai alamat email: tolak dan bungkus galat aslinya.
	if _, err := mail.ParseAddress(s.From); err != nil {
		return SMTP{}, fmt.Errorf("SMTP_FROM is not a valid address: %w", err)
	}
	// Port wajib diisi.
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		return SMTP{}, fmt.Errorf("missing required env var: SMTP_PORT")
	}
	// Port harus bilangan bulat dalam rentang 1 sampai 65535.
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return SMTP{}, fmt.Errorf("SMTP_PORT must be a port number, got %q", port)
	}
	s.Port = n
	// Mode TLS tidak diisi: pakai none (cocok untuk Mailpit di dev).
	if s.TLS == "" {
		s.TLS = "none"
	}
	// Pilih mode TLS yang dikenal. Nilai lain ditolak.
	switch s.TLS {
	// Tiga mode yang diizinkan: tidak ada tindakan tambahan.
	case "none", "starttls", "tls":
	// Mode lain tidak dikenal: tolak.
	default:
		return SMTP{}, fmt.Errorf(`SMTP_TLS must be "none", "starttls" or "tls", got %q`, s.TLS)
	}
	// User dan password harus sama-sama diisi atau sama-sama kosong.
	if (s.User == "") != (s.Password == "") {
		return SMTP{}, fmt.Errorf("SMTP_USER and SMTP_PASSWORD must be set together")
	}
	return s, nil
}
