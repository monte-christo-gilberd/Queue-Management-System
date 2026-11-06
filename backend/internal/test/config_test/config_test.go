package config_test

import (
	"strings"
	"testing"
	"time"

	"qms/internal/config"
)

const strong = "0123456789abcdef0123456789abcdef-strong"

func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	base := map[string]string{
		"APP_ENV": "production", "DATABASE_URL": "postgres://u:p@db/x",
		"JWT_SECRET": strong, "QR_SERVICE_TOKEN": strong,
		"ALLOWED_ORIGINS": "https://qms.example", "PUBLIC_APP_URL": "https://qms.example",
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
		"default jwt":     {"JWT_SECRET": "change-me-jwt-secret"},
		"short jwt":       {"JWT_SECRET": "short"},
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
		"APP_ENV": "", "JWT_SECRET": "change-me-jwt-secret",
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

func TestEmailVerificationRequired(t *testing.T) {
	cases := map[string]struct {
		env     map[string]string
		want    bool
		wantErr string
	}{
		"default ikut mail-service": {env: map[string]string{"MAIL_SERVICE_URL": "http://mail:8092", "MAIL_SERVICE_TOKEN": strong}, want: true},
		"default tanpa mail":        {env: map[string]string{}, want: false},
		"dimatikan eksplisit":       {env: map[string]string{"MAIL_SERVICE_URL": "http://mail:8092", "MAIL_SERVICE_TOKEN": strong, "EMAIL_VERIFICATION_REQUIRED": "false"}, want: false},
		"dinyalakan eksplisit":      {env: map[string]string{"MAIL_SERVICE_URL": "http://mail:8092", "MAIL_SERVICE_TOKEN": strong, "EMAIL_VERIFICATION_REQUIRED": "true"}, want: true},
		"nyala tanpa mail ditolak":  {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "true"}, wantErr: "MAIL_SERVICE_URL"},
		"bukan boolean ditolak":     {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "mungkin"}, wantErr: "EMAIL_VERIFICATION_REQUIRED"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// t.Setenv pada nilai kosong tetap mengosongkan variabel dari lingkungan luar
			for _, k := range []string{"MAIL_SERVICE_URL", "MAIL_SERVICE_TOKEN", "EMAIL_VERIFICATION_REQUIRED"} {
				t.Setenv(k, "")
			}
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
