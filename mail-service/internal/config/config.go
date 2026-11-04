// Package config membaca konfigurasi mail-service dari environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// MinTokenLen: panjang minimum token layanan, sama dengan yang dipakai untuk QR_SERVICE_TOKEN.
const MinTokenLen = 32

type Config struct {
	Port         string
	ServiceToken string
	SMTP         SMTP
	Outbox       Outbox
}

// Outbox: pengiriman email dari tabel notification_outbox. Tanpa DATABASE_URL worker mati dan layanan hanya
// melayani POST /internal/v1/send.
type Outbox struct {
	DatabaseURL  string
	PublicOrigin string
	Rate         float64
	MaxAttempts  int
}

// Enabled: worker outbox hanya jalan bila database dikonfigurasi.
func (o Outbox) Enabled() bool { return o.DatabaseURL != "" }

// Load gagal cepat bila token layanan atau SMTP tidak lengkap atau tidak masuk akal.
func Load() (Config, error) {
	cfg := Config{
		Port:         strings.TrimSpace(os.Getenv("PORT")),
		ServiceToken: os.Getenv("MAIL_SERVICE_TOKEN"),
	}
	if cfg.Port == "" {
		cfg.Port = "8092"
	}
	if len(cfg.ServiceToken) < MinTokenLen {
		return Config{}, fmt.Errorf("MAIL_SERVICE_TOKEN must be set and at least %d characters", MinTokenLen)
	}
	smtp, err := LoadSMTP()
	if err != nil {
		return Config{}, err
	}
	cfg.SMTP = smtp
	ob, err := loadOutbox()
	if err != nil {
		return Config{}, err
	}
	cfg.Outbox = ob
	return cfg, nil
}

func loadOutbox() (Outbox, error) {
	ob := Outbox{
		DatabaseURL:  strings.TrimSpace(os.Getenv("DATABASE_URL")),
		PublicOrigin: strings.TrimRight(strings.TrimSpace(os.Getenv("QMS_PUBLIC_ORIGIN")), "/"),
		Rate:         5,
		MaxAttempts:  8,
	}
	if v := strings.TrimSpace(os.Getenv("OUTBOX_RATE")); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || f > 100 {
			return Outbox{}, fmt.Errorf("OUTBOX_RATE must be a number in (0, 100], got %q", v)
		}
		ob.Rate = f
	}
	if v := strings.TrimSpace(os.Getenv("OUTBOX_MAX_ATTEMPTS")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			return Outbox{}, fmt.Errorf("OUTBOX_MAX_ATTEMPTS must be an integer in [1, 50], got %q", v)
		}
		ob.MaxAttempts = n
	}
	if !ob.Enabled() {
		return ob, nil
	}
	// Tautan di email harus absolut; tanpa itu pelanggan menerima tautan rusak.
	u, err := url.Parse(ob.PublicOrigin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Outbox{}, fmt.Errorf("QMS_PUBLIC_ORIGIN must be an absolute http(s) URL when DATABASE_URL is set, got %q", ob.PublicOrigin)
	}
	return ob, nil
}
