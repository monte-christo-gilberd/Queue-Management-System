// Package config membaca konfigurasi payment-service dari environment.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// MinSecretLen: panjang minimum JWT_SECRET di produksi (sama dengan backend).
const MinSecretLen = 32

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	PublicOrigin string // origin publik app, tujuan kembali dari Stripe Checkout
	Production   bool

	// Stripe. Kosong/placeholder = pembayaran dinonaktifkan: service tetap hidup (health OK) dan checkout
	// menjawab 503, supaya salah konfigurasi di sini tidak pernah menjatuhkan bagian lain dari stack.
	StripeKey     string
	StripeBaseURL string // kosong = api.stripe.com; dev: emulator
	WebhookSecret string
	PriceID       string // opsional; kosong = harga inline dari Amount + Currency

	Amount   int64  // harga satu bulan, satuan utama mata uang (mis. 50000 = Rp50.000)
	Currency string // huruf kecil, kode ISO 4217
}

// StripeConfigured: kunci dan secret webhook terisi dengan nilai sungguhan (bukan contoh).
func (c Config) StripeConfigured() bool {
	return usable(c.StripeKey) && usable(c.WebhookSecret)
}

// usable: nilai terisi dan bukan placeholder dari .env.example.
func usable(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	l := strings.ToLower(v)
	return !strings.HasPrefix(l, "ganti") && !strings.HasPrefix(l, "change-me")
}

// Load gagal cepat hanya untuk hal yang membuat service tidak bisa berfungsi sama sekali (DB, JWT, origin).
// Kunci Stripe yang kosong BUKAN galat fatal; lihat StripeConfigured.
func Load() (Config, error) {
	cfg := Config{
		Port:          strings.TrimSpace(os.Getenv("PORT")),
		DatabaseURL:   strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		PublicOrigin:  strings.TrimRight(strings.TrimSpace(os.Getenv("QMS_PUBLIC_ORIGIN")), "/"),
		Production:    strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production"),
		StripeKey:     strings.TrimSpace(os.Getenv("STRIPE_API_KEY")),
		StripeBaseURL: strings.TrimSpace(os.Getenv("STRIPE_API_BASE_URL")),
		WebhookSecret: strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		PriceID:       strings.TrimSpace(os.Getenv("STRIPE_PRICE_ID")),
		Amount:        50000,
		Currency:      "idr",
	}
	if cfg.Port == "" {
		cfg.Port = "8093"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must be set")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set")
	}
	if cfg.Production && len(cfg.JWTSecret) < MinSecretLen {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least %d characters when APP_ENV=production", MinSecretLen)
	}
	u, err := url.Parse(cfg.PublicOrigin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("QMS_PUBLIC_ORIGIN must be an absolute http(s) URL, got %q", cfg.PublicOrigin)
	}
	if cfg.Production && u.Scheme != "https" {
		return Config{}, fmt.Errorf("QMS_PUBLIC_ORIGIN must be https when APP_ENV=production, got %q", cfg.PublicOrigin)
	}
	if cfg.StripeBaseURL != "" {
		b, err := url.Parse(cfg.StripeBaseURL)
		if err != nil || (b.Scheme != "http" && b.Scheme != "https") || b.Host == "" {
			return Config{}, fmt.Errorf("STRIPE_API_BASE_URL must be an absolute http(s) URL, got %q", cfg.StripeBaseURL)
		}
	}
	if v := strings.TrimSpace(os.Getenv("SUBSCRIPTION_PRICE_AMOUNT")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > 1_000_000_000 {
			return Config{}, fmt.Errorf("SUBSCRIPTION_PRICE_AMOUNT must be a positive integer, got %q", v)
		}
		cfg.Amount = n
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("SUBSCRIPTION_CURRENCY"))); v != "" {
		if len(v) != 3 {
			return Config{}, fmt.Errorf("SUBSCRIPTION_CURRENCY must be a 3-letter ISO code, got %q", v)
		}
		cfg.Currency = v
	}
	return cfg, nil
}
