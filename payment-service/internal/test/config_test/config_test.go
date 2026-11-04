package config_test

import (
	"strings"
	"testing"

	"qms/payment-service/internal/config"
)

func setBase(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgresql://payment_rw:x@db:5432/qms_db")
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("QMS_PUBLIC_ORIGIN", "https://qms.example.com")
	for _, k := range []string{"APP_ENV", "STRIPE_API_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_API_BASE_URL", "STRIPE_PRICE_ID",
		"SUBSCRIPTION_PRICE_AMOUNT", "SUBSCRIPTION_CURRENCY", "PORT"} {
		t.Setenv(k, "")
	}
}

func TestDefaults(t *testing.T) {
	setBase(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "8093" || cfg.Amount != 50000 || cfg.Currency != "idr" || cfg.StripeConfigured() {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestMissingStripeKeysIsNotFatal(t *testing.T) {
	setBase(t)
	t.Setenv("APP_ENV", "production")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("missing Stripe keys must not stop the service: %v", err)
	}
	if cfg.StripeConfigured() {
		t.Fatal("StripeConfigured must be false")
	}
}

func TestPlaceholderKeysAreNotConfigured(t *testing.T) {
	setBase(t)
	t.Setenv("STRIPE_API_KEY", "GANTI_sk_live")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "change-me")
	cfg, _ := config.Load()
	if cfg.StripeConfigured() {
		t.Fatal("placeholder values must not count as configured")
	}
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	cfg, _ = config.Load()
	if !cfg.StripeConfigured() {
		t.Fatal("real-looking values must count as configured")
	}
}

func TestProductionRules(t *testing.T) {
	setBase(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "short")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("err = %v", err)
	}
	setBase(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("QMS_PUBLIC_ORIGIN", "http://qms.example.com")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequiredAndInvalid(t *testing.T) {
	for k, v := range map[string]string{
		"DATABASE_URL": "", "JWT_SECRET": "", "QMS_PUBLIC_ORIGIN": "not a url",
		"STRIPE_API_BASE_URL": "nope", "SUBSCRIPTION_PRICE_AMOUNT": "-5", "SUBSCRIPTION_CURRENCY": "rupiah",
	} {
		setBase(t)
		t.Setenv(k, v)
		if _, err := config.Load(); err == nil {
			t.Errorf("%s=%q: expected error", k, v)
		}
	}
}
