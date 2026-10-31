package config_test

import (
	"strings"
	"testing"

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
