package config_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qms/internal/config"
)

const strong = "0123456789abcdef0123456789abcdef-strong"

// writePEM menulis blok PEM bertipe typ berisi der ke berkas sementara dan mengembalikan path-nya.
func writePEM(t *testing.T, typ string, der []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

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
	return writePEM(t, "PUBLIC KEY", der)
}

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	base := map[string]string{
		"APP_ENV": "production", "DATABASE_URL": "postgres://u:p@db/x",
		"JWT_PUBLIC_KEY_FILE": publicKeyFile(t),
		"QR_SERVICE_TOKEN":    strong,
		"ALLOWED_ORIGINS":     "https://qms.example", "PUBLIC_APP_URL": "https://qms.example",
		"QR_SERVICE_URL": "http://qr:8090",
	}
	for k, v := range kv {
		base[k] = v
	}
	for k, v := range base {
		t.Setenv(k, v)
	}
}

func TestProductionValid(t *testing.T) {
	setEnv(t, nil)
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestProductionRejects(t *testing.T) {
	cases := map[string]map[string]string{
		"default token":   {"QR_SERVICE_TOKEN": "change-me-service-token-at-least-32-bytes"},
		"wildcard origin": {"ALLOWED_ORIGINS": "*"},
		"http origin":     {"ALLOWED_ORIGINS": "http://qms.example"},
		"http public url": {"PUBLIC_APP_URL": "http://qms.example"},
	}
	for name, kv := range cases {
		t.Run(name, func(t *testing.T) {
			setEnv(t, kv)
			_, err := config.Load()
			if err == nil || !strings.Contains(err.Error(), "production") {
				t.Fatalf("expected production error, got %v", err)
			}
		})
	}
}

func TestDevAllowsDefaults(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV":         "",
		"ALLOWED_ORIGINS": "http://localhost:8080", "PUBLIC_APP_URL": "http://localhost:8080",
	})
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownTimeout(t *testing.T) {
	cases := map[string]struct {
		raw     string
		want    time.Duration
		wantErr bool
	}{
		"kosong = 8 detik":  {raw: "", want: 8 * time.Second},
		"diisi":             {raw: "5", want: 5 * time.Second},
		"nol = bawaan":      {raw: "0", want: 8 * time.Second},
		"negatif = bawaan":  {raw: "-3", want: 8 * time.Second},
		"bukan angka = err": {raw: "abc", wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			setEnv(t, map[string]string{"SHUTDOWN_TIMEOUT": tc.raw})
			cfg, err := config.Load()
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
					t.Fatalf("expected SHUTDOWN_TIMEOUT error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ShutdownTimeout != tc.want {
				t.Fatalf("ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, tc.want)
			}
		})
	}
}

func TestJWTPublicKey(t *testing.T) {
	// ecdsa: kunci sah tetapi bukan RSA, harus ditolak.
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalPKIXPublicKey(&ec.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	notPEM := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(notPEM, []byte("bukan pem"), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]struct {
		path    string
		wantErr string
	}{
		"kosong":        {path: "", wantErr: "JWT_PUBLIC_KEY_FILE"},
		"berkas hilang": {path: filepath.Join(t.TempDir(), "tidak-ada.pem"), wantErr: "JWT_PUBLIC_KEY_FILE"},
		"bukan PEM":     {path: notPEM, wantErr: "not a PEM file"},
		"bukan RSA":     {path: writePEM(t, "PUBLIC KEY", ecDER), wantErr: "not RSA"},
		"sah":           {path: publicKeyFile(t)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			setEnv(t, map[string]string{"APP_ENV": "", "JWT_PUBLIC_KEY_FILE": tc.path})
			cfg, err := config.Load()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.JWTPublicKey == nil {
				t.Fatal("JWTPublicKey is nil")
			}
		})
	}
}

func TestEmailVerificationRequired(t *testing.T) {
	// Backend tidak lagi punya konfigurasi mail; ia hanya memutuskan apakah guard verifikasi dipasang.
	cases := map[string]struct {
		env     map[string]string
		want    bool
		wantErr string
	}{
		"default menyala (aman)": {env: map[string]string{}, want: true},
		"dimatikan eksplisit":    {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "false"}, want: false},
		"dinyalakan eksplisit":   {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "true"}, want: true},
		"bukan boolean ditolak":  {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "mungkin"}, wantErr: "EMAIL_VERIFICATION_REQUIRED"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// t.Setenv pada nilai kosong tetap mengosongkan variabel dari lingkungan luar
			t.Setenv("EMAIL_VERIFICATION_REQUIRED", "")
			setEnv(t, tc.env)
			cfg, err := config.Load()
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.EmailVerificationRequired != tc.want {
				t.Fatalf("EmailVerificationRequired = %v, want %v", cfg.EmailVerificationRequired, tc.want)
			}
		})
	}
}
