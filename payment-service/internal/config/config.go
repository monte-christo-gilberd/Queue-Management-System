// Package config membaca konfigurasi payment-service dari environment variable.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// minSecretLen: panjang minimum untuk token layanan dan token webhook.
const minSecretLen = 32

// Config: seluruh konfigurasi payment-service.
type Config struct {
	// Port: port HTTP tempat layanan mendengarkan (bawaan 8095).
	Port string
	// DatabaseURL: DSN PostgreSQL untuk tabel payment_orders, owners, dan payments.
	DatabaseURL string
	// BorderPayBaseURL: alamat dasar BorderPay tanpa /api/v1, mis. https://borderpay.id.
	BorderPayBaseURL string
	// BorderPayAPIKey: API key BorderPay (bp_test_ atau bp_live_). Hanya ada di layanan ini.
	BorderPayAPIKey string
	// WebhookToken: verification token project BorderPay (bpt_...), dicocokkan dengan x-borderpay-token.
	WebhookToken string
	// ServiceToken: token yang dipakai backend utama saat memanggil /internal/v1/payments.
	ServiceToken string
	// HTTPTimeout: batas waktu request ke BorderPay.
	HTTPTimeout time.Duration
}

// Load membaca dan memeriksa konfigurasi. Galat berarti layanan tidak boleh dijalankan.
func Load() (Config, error) {
	cfg := Config{
		Port:             getEnv("PORT", "8095"),
		DatabaseURL:      os.Getenv("PAYMENT_DATABASE_URL"),
		BorderPayBaseURL: getEnv("BORDERPAY_BASE_URL", "https://borderpay.id"),
		BorderPayAPIKey:  os.Getenv("BORDERPAY_API_KEY"),
		WebhookToken:     os.Getenv("BORDERPAY_WEBHOOK_TOKEN"),
		ServiceToken:     os.Getenv("PAYMENT_SERVICE_TOKEN"),
		HTTPTimeout:      15 * time.Second,
	}

	var missing []string
	for name, value := range map[string]string{
		"PAYMENT_DATABASE_URL":    cfg.DatabaseURL,
		"BORDERPAY_API_KEY":       cfg.BorderPayAPIKey,
		"BORDERPAY_WEBHOOK_TOKEN": cfg.WebhookToken,
		"PAYMENT_SERVICE_TOKEN":   cfg.ServiceToken,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("config: variabel wajib kosong: %s", strings.Join(missing, ", "))
	}

	// Token layanan dan webhook harus cukup panjang supaya tidak mudah ditebak.
	if len(cfg.WebhookToken) < minSecretLen || len(cfg.ServiceToken) < minSecretLen {
		return Config{}, errors.New("config: BORDERPAY_WEBHOOK_TOKEN dan PAYMENT_SERVICE_TOKEN minimal 32 karakter")
	}
	// Key BorderPay harus berawalan bp_test_ atau bp_live_.
	if !strings.HasPrefix(cfg.BorderPayAPIKey, "bp_test_") && !strings.HasPrefix(cfg.BorderPayAPIKey, "bp_live_") {
		return Config{}, errors.New("config: BORDERPAY_API_KEY harus berawalan bp_test_ atau bp_live_")
	}
	return cfg, nil
}

// getEnv mengembalikan nilai environment, atau nilai bawaan bila kosong.
func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
