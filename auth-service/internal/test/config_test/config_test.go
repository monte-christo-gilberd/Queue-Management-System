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

	"qms/auth-service/internal/config"
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

// privateKeyFile menulis kunci privat RSA (PKCS#8 seperti keluaran `openssl genpkey`, atau PKCS#1) ke berkas sementara.
func privateKeyFile(t *testing.T, pkcs1 bool) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if pkcs1 {
		return writePEM(t, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key))
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return writePEM(t, "PRIVATE KEY", der)
}

// setEnv mengisi environment dasar yang sah (production), lalu menimpa dengan kv. Nilai kosong mengosongkan variabel.
func setEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	base := map[string]string{
		"APP_ENV": "production", "DATABASE_URL": "postgres://u:p@db/x", "AUTH_DATABASE_URL": "",
		"PUBLIC_APP_URL": "https://qms.example", "AUTH_JWT_PRIVATE_KEY_FILE": privateKeyFile(t, false),
		"AUTH_JWT_KID": "", "PORT": "", "AUTH_PORT": "", "SHUTDOWN_TIMEOUT": "",
		"MAIL_SERVICE_URL": "", "MAIL_SERVICE_TOKEN": "", "EMAIL_VERIFICATION_REQUIRED": "",
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
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.JWTPrivateKey == nil || cfg.Port != "8094" || cfg.JWTKeyID != "qms-1" || cfg.PublicAppURL != "https://qms.example" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestPrivateKey(t *testing.T) {
	ec, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalPKCS8PrivateKey(ec)
	if err != nil {
		t.Fatal(err)
	}
	notPEM := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(notPEM, []byte("bukan pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	garbage := writePEM(t, "PRIVATE KEY", []byte("rusak"))

	cases := map[string]struct {
		path    string
		wantErr string
	}{
		"kosong":        {path: "", wantErr: "AUTH_JWT_PRIVATE_KEY_FILE"},
		"berkas hilang": {path: filepath.Join(t.TempDir(), "tidak-ada.pem"), wantErr: "AUTH_JWT_PRIVATE_KEY_FILE"},
		"bukan PEM":     {path: notPEM, wantErr: "not a PEM file"},
		"isi rusak":     {path: garbage, wantErr: "parse private key"},
		"bukan RSA":     {path: writePEM(t, "PRIVATE KEY", ecDER), wantErr: "not RSA"},
		"PKCS#8 sah":    {path: privateKeyFile(t, false)},
		"PKCS#1 sah":    {path: privateKeyFile(t, true)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			setEnv(t, map[string]string{"APP_ENV": "", "AUTH_JWT_PRIVATE_KEY_FILE": tc.path})
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
			if cfg.JWTPrivateKey == nil {
				t.Fatal("JWTPrivateKey is nil")
			}
		})
	}
}

func TestRequiredAndDefaults(t *testing.T) {
	for k, wantErr := range map[string]string{"DATABASE_URL": "DATABASE_URL", "PUBLIC_APP_URL": "PUBLIC_APP_URL"} {
		setEnv(t, map[string]string{k: ""})
		if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("%s kosong: err = %v", k, err)
		}
	}

	// AUTH_DATABASE_URL didahulukan, AUTH_PORT dan AUTH_JWT_KID dibaca, "/" di akhir URL dibuang.
	setEnv(t, map[string]string{"AUTH_DATABASE_URL": "postgres://auth_rw:x@db/y", "AUTH_PORT": "9999", "AUTH_JWT_KID": "kunci-2", "PUBLIC_APP_URL": "https://qms.example///"})
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseURL != "postgres://auth_rw:x@db/y" || cfg.Port != "9999" || cfg.JWTKeyID != "kunci-2" || cfg.PublicAppURL != "https://qms.example" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestProductionRejects(t *testing.T) {
	cases := map[string]map[string]string{
		"http public url":    {"PUBLIC_APP_URL": "http://qms.example"},
		"public url no host": {"PUBLIC_APP_URL": "https://"},
		"short mail token":   {"MAIL_SERVICE_URL": "http://mail:8092", "MAIL_SERVICE_TOKEN": "short"},
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

func TestDevAllowsHTTPAndShortToken(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV": "", "PUBLIC_APP_URL": "http://localhost:8080",
		"MAIL_SERVICE_URL": "http://mail:8092", "MAIL_SERVICE_TOKEN": "change-me",
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

func TestMailSettings(t *testing.T) {
	mail := map[string]string{"MAIL_SERVICE_URL": "http://mail:8092/", "MAIL_SERVICE_TOKEN": strong}
	with := func(kv map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range mail {
			out[k] = v
		}
		for k, v := range kv {
			out[k] = v
		}
		return out
	}

	cases := map[string]struct {
		env     map[string]string
		want    bool
		wantErr string
	}{
		"default ikut mail-service": {env: mail, want: true},
		"default tanpa mail":        {env: map[string]string{}, want: false},
		"dimatikan eksplisit":       {env: with(map[string]string{"EMAIL_VERIFICATION_REQUIRED": "false"}), want: false},
		"dinyalakan eksplisit":      {env: with(map[string]string{"EMAIL_VERIFICATION_REQUIRED": "true"}), want: true},
		"nyala tanpa mail ditolak":  {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "true"}, wantErr: "MAIL_SERVICE_URL"},
		"bukan boolean ditolak":     {env: map[string]string{"EMAIL_VERIFICATION_REQUIRED": "mungkin"}, wantErr: "EMAIL_VERIFICATION_REQUIRED"},
		"hanya URL tanpa token":     {env: map[string]string{"MAIL_SERVICE_URL": "http://mail:8092"}, wantErr: "must be set together"},
		"hanya token tanpa URL":     {env: map[string]string{"MAIL_SERVICE_TOKEN": strong}, wantErr: "must be set together"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
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
			if tc.env["MAIL_SERVICE_URL"] != "" && strings.HasSuffix(cfg.MailServiceURL, "/") {
				t.Fatalf("MailServiceURL harus tanpa '/' di akhir: %q", cfg.MailServiceURL)
			}
		})
	}
}
