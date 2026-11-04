// Package config membaca konfigurasi mail-service dari environment.
package config

import (
	"fmt"
	"os"
	"strings"
)

// MinTokenLen: panjang minimum token layanan, sama dengan yang dipakai untuk QR_SERVICE_TOKEN.
const MinTokenLen = 32

type Config struct {
	Port         string
	ServiceToken string
	SMTP         SMTP
}

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
	return cfg, nil
}
