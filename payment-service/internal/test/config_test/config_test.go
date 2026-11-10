package config_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qms/payment-service/internal/config"
)

// publicKeyFile menulis kunci publik RSA yang sah (format PKIX, seperti keluaran openssl) ke berkas sementara.
func publicKeyFile(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pub.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func setBase(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgresql://payment_rw:x@db:5432/qms_db")
	t.Setenv("JWT_PUBLIC_KEY_FILE", publicKeyFile(t))
	t.Setenv("JWT_LEGACY_HS256_SECRET", "")
	t.Setenv("QMS_PUBLIC_ORIGIN", "https://qms.example.com")
	for _, k := range []string{"APP_ENV", "STRIPE_API_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_API_BASE_URL", "STRIPE_PRICE_ID",
		"SUBSCRIPTION_PRICE_AMOUNT", "SUBSCRIPTION_CURRENCY", "PORT", "DEV_PAYMENT_SIMULATOR"} {
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
	t.Setenv("JWT_LEGACY_HS256_SECRET", "short")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "JWT_LEGACY_HS256_SECRET") {
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
		"DATABASE_URL": "", "JWT_PUBLIC_KEY_FILE": "", "QMS_PUBLIC_ORIGIN": "not a url",
		"STRIPE_API_BASE_URL": "nope", "SUBSCRIPTION_PRICE_AMOUNT": "-5", "SUBSCRIPTION_CURRENCY": "rupiah",
	} {
		setBase(t)
		t.Setenv(k, v)
		if _, err := config.Load(); err == nil {
			t.Errorf("%s=%q: expected error", k, v)
		}
	}
}

func TestSimulatorOffByDefault(t *testing.T) {
	setBase(t)
	cfg, err := config.Load()
	if err != nil || cfg.DevSimulator {
		t.Fatalf("cfg.DevSimulator=%v err=%v, want off", cfg.DevSimulator, err)
	}
}

func TestSimulatorOnInDev(t *testing.T) {
	setBase(t)
	t.Setenv("QMS_PUBLIC_ORIGIN", "http://localhost:8080")
	t.Setenv("DEV_PAYMENT_SIMULATOR", "true")
	cfg, err := config.Load()
	if err != nil || !cfg.DevSimulator {
		t.Fatalf("cfg.DevSimulator=%v err=%v, want on", cfg.DevSimulator, err)
	}
}

// Pembayaran palsu tidak boleh menyala di produksi, apa pun nilai lainnya.
func TestSimulatorRefusedInProduction(t *testing.T) {
	setBase(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DEV_PAYMENT_SIMULATOR", "true")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "DEV_PAYMENT_SIMULATOR") {
		t.Fatalf("err = %v, want refusal", err)
	}
}

func TestSimulatorAndStripeKeysAreExclusive(t *testing.T) {
	setBase(t)
	t.Setenv("QMS_PUBLIC_ORIGIN", "http://localhost:8080")
	t.Setenv("DEV_PAYMENT_SIMULATOR", "1")
	t.Setenv("STRIPE_API_KEY", "sk_test_123")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_123")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("err = %v", err)
	}
}
