// Package config membaca dan memvalidasi konfigurasi qr-service dari environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DefaultDynamicTTL adalah masa berlaku token join dinamis bila tidak diatur lewat env.
const DefaultDynamicTTL = 120 * time.Second

const (
	minDynamicTTL = 30 * time.Second
	maxDynamicTTL = 10 * time.Minute
)

// Config adalah konfigurasi runtime qr-service. Service ini stateless: tidak ada
// database, hanya secret untuk menandatangani token.
type Config struct {
	Port         string
	JoinSecret   string
	DynamicTTL   time.Duration
	ServiceToken string
	LogLevel     string
}

// Load membaca environment. Variabel wajib yang kosong menghasilkan error agar
// container gagal start cepat daripada melayani token yang tidak bisa dipakai.
func Load() (Config, error) {
	cfg := Config{
		Port:         envOr("PORT", "8090"),
		JoinSecret:   os.Getenv("JOIN_LINK_SECRET"),
		ServiceToken: os.Getenv("QR_SERVICE_TOKEN"),
		LogLevel:     envOr("LOG_LEVEL", "info"),
	}

	if cfg.JoinSecret == "" {
		return Config{}, fmt.Errorf("missing required env var: JOIN_LINK_SECRET")
	}
	if len(cfg.JoinSecret) < 32 {
		return Config{}, fmt.Errorf("JOIN_LINK_SECRET must be at least 32 bytes")
	}
	if cfg.ServiceToken == "" {
		return Config{}, fmt.Errorf("missing required env var: QR_SERVICE_TOKEN")
	}

	ttl := DefaultDynamicTTL
	if raw := os.Getenv("QR_DYNAMIC_TTL_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid QR_DYNAMIC_TTL_SECONDS: %w", err)
		}
		ttl = time.Duration(seconds) * time.Second
	}
	if ttl < minDynamicTTL || ttl > maxDynamicTTL {
		return Config{}, fmt.Errorf(
			"QR_DYNAMIC_TTL_SECONDS must be between %d and %d",
			int(minDynamicTTL.Seconds()), int(maxDynamicTTL.Seconds()),
		)
	}
	cfg.DynamicTTL = ttl

	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
