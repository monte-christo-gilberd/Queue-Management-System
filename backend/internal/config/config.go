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
	// Env: "production" mengaktifkan validasi ketat (APP_ENV).
	Env            string
	DatabaseURL    string
	JWTSecret      string
	Port           string
	AllowedOrigins []string
	// TrustedProxies: IP/CIDR proxy yang boleh menentukan IP klien (X-Forwarded-For). Kosong = tidak ada.
	TrustedProxies []string

	// QR microservice (dipakai backend untuk memanggil /internal/v1/*).
	PublicAppURL   string
	QRServiceURL   string
	QRServiceToken string
	QRHTTPTimeout  time.Duration

	// mail-service (reset password). Opsional sebagai pasangan: keduanya diisi atau keduanya kosong. Kosong = fitur
	// lupa password tidak dipasang (rute 404).
	MailServiceURL   string
	MailServiceToken string

	// Kunci VAPID publik untuk Web Push (opsional; kosong = fitur push mati).
	VAPIDPublicKey string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Env:            strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		Port:           os.Getenv("PORT"),
		AllowedOrigins: (parseOrigins(os.Getenv("ALLOWED_ORIGINS"))),
		TrustedProxies: parseOrigins(os.Getenv("TRUSTED_PROXIES")),

		PublicAppURL:   strings.TrimRight(os.Getenv("PUBLIC_APP_URL"), "/"),
		QRServiceURL:   strings.TrimRight(os.Getenv("QR_SERVICE_URL"), "/"),
		QRServiceToken: os.Getenv("QR_SERVICE_TOKEN"),
		VAPIDPublicKey: os.Getenv("VAPID_PUBLIC_KEY"),

		MailServiceURL:   strings.TrimRight(os.Getenv("MAIL_SERVICE_URL"), "/"),
		MailServiceToken: os.Getenv("MAIL_SERVICE_TOKEN"),
	}
	if (cfg.MailServiceURL == "") != (cfg.MailServiceToken == "") {
		return Config{}, fmt.Errorf("MAIL_SERVICE_URL and MAIL_SERVICE_TOKEN must be set together")
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

	if cfg.Env == "production" {
		if err := cfg.validateProduction(); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

var weakSecrets = map[string]bool{
	"change-me-jwt-secret":                           true,
	"change-me-service-token-at-least-32-bytes":      true,
	"change-me-mail-service-token-at-least-32-bytes": true,
	"secret":   true,
	"password": true,
}

// validateProduction menolak konfigurasi yang tidak aman saat APP_ENV=production.
func (c Config) validateProduction() error {
	if len(c.JWTSecret) < 32 || weakSecrets[c.JWTSecret] {
		return fmt.Errorf("production: JWT_SECRET must be at least 32 chars and not a default value")
	}
	if len(c.QRServiceToken) < 32 || weakSecrets[c.QRServiceToken] {
		return fmt.Errorf("production: QR_SERVICE_TOKEN must be at least 32 chars and not a default value")
	}
	if c.MailServiceToken != "" && (len(c.MailServiceToken) < 32 || weakSecrets[c.MailServiceToken]) {
		return fmt.Errorf("production: MAIL_SERVICE_TOKEN must be at least 32 chars and not a default value")
	}
	for _, o := range c.AllowedOrigins {
		if o == "*" {
			return fmt.Errorf("production: ALLOWED_ORIGINS must not contain *")
		}
		if !strings.HasPrefix(o, "https://") {
			return fmt.Errorf("production: ALLOWED_ORIGINS must use https (got %q)", o)
		}
	}
	if !strings.HasPrefix(c.PublicAppURL, "https://") {
		return fmt.Errorf("production: PUBLIC_APP_URL must use https")
	}
	if err := validatePublicURL(c.PublicAppURL); err != nil {
		return fmt.Errorf("production: PUBLIC_APP_URL: %w", err)
	}
	return nil
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
