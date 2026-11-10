// Package config membaca dan memvalidasi konfigurasi auth-service dari environment variable.
//
// Wajib: DATABASE_URL (atau AUTH_DATABASE_URL), PUBLIC_APP_URL, AUTH_JWT_PRIVATE_KEY_FILE.
// Opsional: MAIL_SERVICE_URL dan MAIL_SERVICE_TOKEN (keduanya atau tidak sama sekali), EMAIL_VERIFICATION_REQUIRED,
// AUTH_JWT_KID, PORT (bawaan 8094), SHUTDOWN_TIMEOUT (bawaan 8 detik), APP_ENV.
package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// DefaultPort: port HTTP bawaan auth-service (lihat tabel port di docker-compose.yml).
const DefaultPort = "8094"

// Config: pengaturan auth-service setelah Load.
type Config struct {
	// Env: APP_ENV. "production" mengaktifkan pemeriksaan keamanan tambahan.
	Env string
	// DatabaseURL: koneksi PostgreSQL dengan role auth_rw.
	DatabaseURL string
	// Port: port HTTP tempat server listen.
	Port string
	// PublicAppURL: alamat publik aplikasi; dipakai untuk tautan di email.
	PublicAppURL string
	// JWTPrivateKey: kunci privat RSA untuk menandatangani JWT RS256. Hanya ada di auth-service.
	JWTPrivateKey *rsa.PrivateKey
	// JWTKeyID: nilai header `kid` pada token, untuk rotasi kunci nanti.
	JWTKeyID string
	// MailServiceURL dan MailServiceToken: klien ke mail-service. Kosong = email mati (reset dan verifikasi tidak dipasang).
	MailServiceURL   string
	MailServiceToken string
	// EmailVerificationRequired: email baru dikirimi tautan verifikasi saat registrasi.
	EmailVerificationRequired bool
	// ShutdownTimeout: batas menunggu request berjalan saat berhenti.
	ShutdownTimeout time.Duration
}

// Load membaca environment (dengan ../.env bila ada), memvalidasi, dan mengembalikan Config.
// Error berarti server tidak boleh dijalankan.
func Load() (Config, error) {
	// muat .env di akar proyek bila ada (di container tidak ada); variabel yang sudah ada tidak ditimpa
	_ = godotenv.Load("../.env")

	// kunci privat wajib ada: tanpa itu tidak ada token yang bisa diterbitkan
	keyPath := strings.TrimSpace(os.Getenv("AUTH_JWT_PRIVATE_KEY_FILE"))
	if keyPath == "" {
		return Config{}, errors.New("missing required env var: AUTH_JWT_PRIVATE_KEY_FILE")
	}
	key, err := loadPrivateKey(keyPath)
	if err != nil {
		return Config{}, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_FILE: %w", err)
	}

	// isi Config dari environment; "/" di akhir URL dibuang agar tidak menghasilkan "//" saat digabung path
	cfg := Config{
		Env:              strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		DatabaseURL:      firstEnv("AUTH_DATABASE_URL", "DATABASE_URL"),
		Port:             firstEnv("AUTH_PORT", "PORT"),
		PublicAppURL:     strings.TrimRight(os.Getenv("PUBLIC_APP_URL"), "/"),
		JWTPrivateKey:    key,
		JWTKeyID:         firstEnv("AUTH_JWT_KID"),
		MailServiceURL:   strings.TrimRight(os.Getenv("MAIL_SERVICE_URL"), "/"),
		MailServiceToken: os.Getenv("MAIL_SERVICE_TOKEN"),
		ShutdownTimeout:  8 * time.Second,
	}

	// DATABASE_URL wajib ada
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("missing required env var: DATABASE_URL")
	}
	// PUBLIC_APP_URL wajib ada: dipakai membuat tautan di email
	if cfg.PublicAppURL == "" {
		return Config{}, errors.New("missing required env var: PUBLIC_APP_URL")
	}
	// PORT kosong = 8094
	if cfg.Port == "" {
		cfg.Port = DefaultPort
	}
	// kid kosong = qms-1
	if cfg.JWTKeyID == "" {
		cfg.JWTKeyID = "qms-1"
	}
	// MAIL_SERVICE_URL dan MAIL_SERVICE_TOKEN harus sama-sama diisi atau sama-sama kosong
	if (cfg.MailServiceURL == "") != (cfg.MailServiceToken == "") {
		return Config{}, errors.New("MAIL_SERVICE_URL and MAIL_SERVICE_TOKEN must be set together")
	}

	// EMAIL_VERIFICATION_REQUIRED: kosong = ikut ada-tidaknya mail-service. Bukan boolean = error.
	cfg.EmailVerificationRequired = cfg.MailServiceURL != ""
	if raw := strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_REQUIRED")); raw != "" {
		required, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid EMAIL_VERIFICATION_REQUIRED: %w", err)
		}
		cfg.EmailVerificationRequired = required
	}
	// verifikasi tanpa pengirim email akan mengunci semua user baru, jadi ditolak sejak start
	if cfg.EmailVerificationRequired && cfg.MailServiceURL == "" {
		return Config{}, errors.New("EMAIL_VERIFICATION_REQUIRED needs MAIL_SERVICE_URL and MAIL_SERVICE_TOKEN")
	}

	// SHUTDOWN_TIMEOUT dalam detik. Kosong atau <= 0 = 8 detik. Bukan angka = error.
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
		}
		if seconds > 0 {
			cfg.ShutdownTimeout = time.Duration(seconds) * time.Second
		}
	}

	// validasi tambahan khusus production
	if cfg.Env == "production" {
		if err := cfg.validateProduction(); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

// validateProduction menolak konfigurasi tidak aman saat APP_ENV=production.
func (c Config) validateProduction() error {
	// tautan di email harus https agar token tidak lewat jaringan terbuka
	if !strings.HasPrefix(c.PublicAppURL, "https://") {
		return errors.New("production: PUBLIC_APP_URL must use https")
	}
	// harus URL absolut dengan host
	if u, err := url.Parse(c.PublicAppURL); err != nil || u.Host == "" {
		return errors.New("production: PUBLIC_APP_URL must be an absolute URL with a host")
	}
	// token antarservice ke mail-service harus cukup panjang
	if c.MailServiceToken != "" && len(c.MailServiceToken) < 32 {
		return errors.New("production: MAIL_SERVICE_TOKEN must be at least 32 chars")
	}
	return nil
}

// loadPrivateKey membaca kunci privat RSA dari berkas PEM (PKCS#8 atau PKCS#1).
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	// baca isi berkas kunci
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// decode blok PEM
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("not a PEM file")
	}
	// format PKCS#8 (bawaan openssl genpkey) didahulukan
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsaKey, nil
	}
	// cadangan: format PKCS#1 ("RSA PRIVATE KEY")
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return key, nil
}

// firstEnv mengembalikan nilai variabel pertama yang tidak kosong.
func firstEnv(keys ...string) string {
	// kembalikan nilai pertama yang terisi, sehingga AUTH_* mengalahkan nama umum seperti PORT
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
