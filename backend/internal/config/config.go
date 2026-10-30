package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	JWTSecret      string
	Port           string
	AllowedOrigins []string
	// TrustedProxies: IP/CIDR proxy yang boleh menentukan IP klien (X-Forwarded-For). Kosong = tidak ada.
	TrustedProxies []string

	// QR microservice (dipakai backend untuk memanggil /internal/v1/*).
	PublicAppURL   string
	PublicQRURL    string
	QRServiceURL   string
	QRServiceToken string
	QRHTTPTimeout  time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		Port:           os.Getenv("PORT"),
		AllowedOrigins: (parseOrigins(os.Getenv("ALLOWED_ORIGINS"))),
		TrustedProxies: parseOrigins(os.Getenv("TRUSTED_PROXIES")),

		PublicAppURL:   strings.TrimRight(os.Getenv("PUBLIC_APP_URL"), "/"),
		PublicQRURL:    strings.TrimRight(os.Getenv("PUBLIC_QR_URL"), "/"),
		QRServiceURL:   strings.TrimRight(os.Getenv("QR_SERVICE_URL"), "/"),
		QRServiceToken: os.Getenv("QR_SERVICE_TOKEN"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("missing required env var: DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("missing required env var: JWT_SECRET")
	}
	if cfg.AllowedOrigins == nil {
		return Config{}, fmt.Errorf("missing required env var: ALLOWED_ORIGINS")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	if cfg.PublicAppURL == "" {
		return Config{}, fmt.Errorf("missing required env var: PUBLIC_APP_URL")
	}
	if cfg.PublicQRURL == "" {
		return Config{}, fmt.Errorf("missing required env var: PUBLIC_QR_URL")
	}
	if cfg.QRServiceURL == "" {
		return Config{}, fmt.Errorf("missing required env var: QR_SERVICE_URL")
	}
	if cfg.QRServiceToken == "" {
		return Config{}, fmt.Errorf("missing required env var: QR_SERVICE_TOKEN")
	}

	timeout := time.Second
	if raw := os.Getenv("QR_HTTP_TIMEOUT"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid QR_HTTP_TIMEOUT: %w", err)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	cfg.QRHTTPTimeout = timeout

	return cfg, nil
}

func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func validatePublicURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid public URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("public URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("public URL must include a host")
	}
	return nil
}
