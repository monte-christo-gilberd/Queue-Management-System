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
	Host     string
	Port     int
	From     string // alamat pengirim, mis. "QMS <no-reply@contoh.id>"
	User     string
	Password string
	TLS      string
}

// Addr: host:port untuk dial.
func (s SMTP) Addr() string { return fmt.Sprintf("%s:%d", s.Host, s.Port) }

// LoadSMTP membaca SMTP_* dan gagal cepat bila tidak lengkap atau tidak masuk akal, sama seperti VAPID:
// lebih baik service menolak start daripada email diam-diam tidak pernah terkirim.
func LoadSMTP() (SMTP, error) {
	s := SMTP{
		Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
		From:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		TLS:      strings.ToLower(strings.TrimSpace(os.Getenv("SMTP_TLS"))),
	}
	if s.Host == "" {
		return SMTP{}, fmt.Errorf("missing required env var: SMTP_HOST")
	}
	if s.From == "" {
		return SMTP{}, fmt.Errorf("missing required env var: SMTP_FROM")
	}
	if _, err := mail.ParseAddress(s.From); err != nil {
		return SMTP{}, fmt.Errorf("SMTP_FROM is not a valid address: %w", err)
	}
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		return SMTP{}, fmt.Errorf("missing required env var: SMTP_PORT")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return SMTP{}, fmt.Errorf("SMTP_PORT must be a port number, got %q", port)
	}
	s.Port = n
	if s.TLS == "" {
		s.TLS = "none"
	}
	switch s.TLS {
	case "none", "starttls", "tls":
	default:
		return SMTP{}, fmt.Errorf(`SMTP_TLS must be "none", "starttls" or "tls", got %q`, s.TLS)
	}
	if (s.User == "") != (s.Password == "") {
		return SMTP{}, fmt.Errorf("SMTP_USER and SMTP_PASSWORD must be set together")
	}
	return s, nil
}
