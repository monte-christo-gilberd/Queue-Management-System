// Package config membaca dan memvalidasi konfigurasi auth-service dari environment variable.
//
// Wajib: DATABASE_URL (atau AUTH_DATABASE_URL), PUBLIC_APP_URL, AUTH_JWT_PRIVATE_KEY_FILE.
// Opsional: MAIL_SERVICE_URL dan MAIL_SERVICE_TOKEN (keduanya atau tidak sama sekali), EMAIL_VERIFICATION_REQUIRED,
// AUTH_JWT_KID, GOOGLE_CLIENT_IDS (kosong = login Google mati), GOOGLE_CERTS_URL, AUTH_PORT atau PORT (bawaan 8094),
// SHUTDOWN_TIMEOUT (bawaan 8 detik), APP_ENV, TRUSTED_PROXIES.
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
	MailServiceURL string
	// MailServiceToken: MAIL_SERVICE_TOKEN, dikirim sebagai X-Service-Token. Di production minimal 32 karakter.
	MailServiceToken string
	// EmailVerificationRequired: email baru dikirimi tautan verifikasi saat registrasi.
	EmailVerificationRequired bool
	// ShutdownTimeout: batas menunggu request berjalan saat berhenti.
	ShutdownTimeout time.Duration
	// GoogleClientIDs: client ID OAuth yang sah (web, Android, iOS) untuk klaim `aud` ID token Google
	// (GOOGLE_CLIENT_IDS, dipisah koma). Kosong = login Google mati: rutenya tidak dipasang (404).
	GoogleClientIDs []string
	// GoogleCertsURL: alamat kunci publik Google (GOOGLE_CERTS_URL). Bawaan alamat resmi Google; hanya diganti di tes.
	GoogleCertsURL string
	// TrustedProxies: daftar CIDR proxy yang boleh menentukan IP klien lewat X-Forwarded-For (TRUSTED_PROXIES,
	// dipisah koma). Kosong = tidak ada proxy yang dipercaya.
	TrustedProxies []string
}

// Load membaca environment (dengan ../.env bila ada), memvalidasi, dan mengembalikan Config.
// Error berarti server tidak boleh dijalankan.
func Load() (Config, error) {
	// muat .env di akar proyek bila ada (di container tidak ada); variabel yang sudah ada tidak ditimpa
	_ = godotenv.Load("../.env")

	// kunci privat wajib ada: tanpa itu tidak ada token yang bisa diterbitkan
	keyPath := strings.TrimSpace(os.Getenv("AUTH_JWT_PRIVATE_KEY_FILE"))
	if keyPath == "" {
		// Server tidak boleh jalan dengan konfigurasi production yang tidak aman.
		// Server tidak jalan tanpa kunci penanda token.
		return Config{}, errors.New("missing required env var: AUTH_JWT_PRIVATE_KEY_FILE")
	}
	// Baca dan parse kunci privat dari berkas.
	key, err := loadPrivateKey(keyPath)
	// Berkas tidak ada atau bukan kunci RSA yang valid: server tidak boleh jalan.
	if err != nil {
		// Galat dibungkus dengan nama variabelnya agar operator tahu apa yang harus diperbaiki.
		return Config{}, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_FILE: %w", err)
	}

	// isi Config dari environment; "/" di akhir URL dibuang agar tidak menghasilkan "//" saat digabung path
	cfg := Config{
		// APP_ENV, huruf kecil agar "Production" dan "production" dianggap sama.
		Env: strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		// AUTH_DATABASE_URL mengalahkan DATABASE_URL.
		DatabaseURL: firstEnv("AUTH_DATABASE_URL", "DATABASE_URL"),
		// AUTH_PORT mengalahkan PORT; kosong diisi bawaan di bawah.
		Port: firstEnv("AUTH_PORT", "PORT"),
		// PUBLIC_APP_URL tanpa "/" di akhir.
		PublicAppURL: strings.TrimRight(os.Getenv("PUBLIC_APP_URL"), "/"),
		// Kunci yang sudah di-parse dari AUTH_JWT_PRIVATE_KEY_FILE.
		JWTPrivateKey: key,
		// AUTH_JWT_KID; bawaan diisi di bawah.
		JWTKeyID: firstEnv("AUTH_JWT_KID"),
		// MAIL_SERVICE_URL tanpa "/" di akhir.
		MailServiceURL: strings.TrimRight(os.Getenv("MAIL_SERVICE_URL"), "/"),
		// MAIL_SERVICE_TOKEN apa adanya.
		MailServiceToken: os.Getenv("MAIL_SERVICE_TOKEN"),
		// Bawaan 8 detik; bisa diganti SHUTDOWN_TIMEOUT di bawah.
		ShutdownTimeout: 8 * time.Second,
		// GOOGLE_CLIENT_IDS dipecah per koma.
		GoogleClientIDs: splitList(os.Getenv("GOOGLE_CLIENT_IDS")),
		// GOOGLE_CERTS_URL; kosong berarti Verifier memakai alamat resmi Google.
		GoogleCertsURL: strings.TrimSpace(os.Getenv("GOOGLE_CERTS_URL")),
		// TRUSTED_PROXIES dipecah per koma.
		TrustedProxies: splitList(os.Getenv("TRUSTED_PROXIES")),
	}

	// DATABASE_URL wajib ada
	if cfg.DatabaseURL == "" {
		// Server tidak jalan tanpa database.
		return Config{}, errors.New("missing required env var: DATABASE_URL")
	}
	// PUBLIC_APP_URL wajib ada: dipakai membuat tautan di email
	if cfg.PublicAppURL == "" {
		// Tautan di email tidak bisa dibuat tanpa alamat publik.
		return Config{}, errors.New("missing required env var: PUBLIC_APP_URL")
	}
	// PORT kosong = 8094
	if cfg.Port == "" {
		// Pakai port bawaan.
		cfg.Port = DefaultPort
	}
	// kid kosong = qms-1
	if cfg.JWTKeyID == "" {
		// Pakai kid bawaan.
		cfg.JWTKeyID = "qms-1"
	}
	// MAIL_SERVICE_URL dan MAIL_SERVICE_TOKEN harus sama-sama diisi atau sama-sama kosong
	if (cfg.MailServiceURL == "") != (cfg.MailServiceToken == "") {
		// Salah satunya saja berarti konfigurasi setengah jadi: ditolak sejak start.
		return Config{}, errors.New("MAIL_SERVICE_URL and MAIL_SERVICE_TOKEN must be set together")
	}

	// EMAIL_VERIFICATION_REQUIRED: kosong = ikut ada-tidaknya mail-service. Bukan boolean = error.
	cfg.EmailVerificationRequired = cfg.MailServiceURL != ""
	// Bila variabel diisi, nilainya menimpa nilai bawaan.
	if raw := strings.TrimSpace(os.Getenv("EMAIL_VERIFICATION_REQUIRED")); raw != "" {
		// Terima true/false, 1/0, dan sejenisnya.
		required, err := strconv.ParseBool(raw)
		// Bukan boolean: ditolak agar salah ketik tidak diam-diam mematikan verifikasi.
		if err != nil {
			// Galat dibungkus dengan nama variabel.
			return Config{}, fmt.Errorf("invalid EMAIL_VERIFICATION_REQUIRED: %w", err)
		}
		// Pakai nilai eksplisit dari environment.
		cfg.EmailVerificationRequired = required
	}
	// verifikasi tanpa pengirim email akan mengunci semua user baru, jadi ditolak sejak start
	if cfg.EmailVerificationRequired && cfg.MailServiceURL == "" {
		// Konfigurasi ditolak: user baru tidak akan bisa memverifikasi email.
		return Config{}, errors.New("EMAIL_VERIFICATION_REQUIRED needs MAIL_SERVICE_URL and MAIL_SERVICE_TOKEN")
	}

	// SHUTDOWN_TIMEOUT dalam detik. Kosong atau <= 0 = 8 detik. Bukan angka = error.
	if raw := strings.TrimSpace(os.Getenv("SHUTDOWN_TIMEOUT")); raw != "" {
		// Detik sebagai bilangan bulat.
		seconds, err := strconv.Atoi(raw)
		// Bukan angka: ditolak agar salah ketik tidak diabaikan.
		if err != nil {
			// Galat dibungkus dengan nama variabel.
			return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
		}
		// Nol atau negatif diabaikan; nilai bawaan 8 detik tetap berlaku.
		if seconds > 0 {
			// Ubah detik menjadi Duration.
			cfg.ShutdownTimeout = time.Duration(seconds) * time.Second
		}
	}

	// validasi tambahan khusus production
	if cfg.Env == "production" {
		// Pemeriksaan keamanan tambahan; galat pertama langsung menggagalkan start.
		if err := cfg.validateProduction(); err != nil {
			return Config{}, err
		}
	}
	// Konfigurasi valid dan siap dipakai.
	return cfg, nil
}

// validateProduction menolak konfigurasi tidak aman saat APP_ENV=production.
func (c Config) validateProduction() error {
	// tautan di email harus https agar token tidak lewat jaringan terbuka
	if !strings.HasPrefix(c.PublicAppURL, "https://") {
		// Ditolak: token di tautan bisa disadap lewat HTTP.
		return errors.New("production: PUBLIC_APP_URL must use https")
	}
	// harus URL absolut dengan host
	if u, err := url.Parse(c.PublicAppURL); err != nil || u.Host == "" {
		// Ditolak: tautan email tidak akan bisa dibuka.
		return errors.New("production: PUBLIC_APP_URL must be an absolute URL with a host")
	}
	// token antarservice ke mail-service harus cukup panjang
	if c.MailServiceToken != "" && len(c.MailServiceToken) < 32 {
		// Ditolak: token pendek mudah ditebak.
		return errors.New("production: MAIL_SERVICE_TOKEN must be at least 32 chars")
	}
	// kunci publik Google tidak boleh diambil lewat jaringan terbuka (kunci palsu = token palsu diterima)
	if c.GoogleCertsURL != "" && !strings.HasPrefix(c.GoogleCertsURL, "https://") {
		// Ditolak: kunci publik yang diambil lewat HTTP bisa disusupi.
		return errors.New("production: GOOGLE_CERTS_URL must use https")
	}
	// Semua pemeriksaan production lolos.
	return nil
}

// splitList memecah daftar yang dipisah koma menjadi slice, membuang spasi dan entri kosong.
func splitList(raw string) []string {
	// Hasil; tetap nil bila tidak ada entri.
	var out []string
	// Pecah per koma.
	for _, p := range strings.Split(raw, ",") {
		// Buang spasi; entri kosong dilewati.
		if p = strings.TrimSpace(p); p != "" {
			// Simpan entri yang terisi.
			out = append(out, p)
		}
	}
	// Kembalikan daftar yang sudah dibersihkan.
	return out
}

// loadPrivateKey membaca kunci privat RSA dari berkas PEM (PKCS#8 atau PKCS#1).
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	// baca isi berkas kunci
	raw, err := os.ReadFile(path)
	if err != nil {
		// Berkas tidak bisa dibaca.
		return nil, err
	}
	// decode blok PEM
	block, _ := pem.Decode(raw)
	if block == nil {
		// Isi berkas bukan format PEM.
		return nil, errors.New("not a PEM file")
	}
	// format PKCS#8 (bawaan openssl genpkey) didahulukan
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		// PKCS#8 bisa berisi jenis kunci lain (EC, Ed25519).
		rsaKey, ok := key.(*rsa.PrivateKey)
		// Hanya RSA yang didukung untuk RS256.
		if !ok {
			// Ditolak: algoritma penanda tidak cocok.
			return nil, errors.New("private key is not RSA")
		}
		// Kunci RSA valid.
		return rsaKey, nil
	}
	// cadangan: format PKCS#1 ("RSA PRIVATE KEY")
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Galat dibungkus dengan konteks.
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	// Kunci RSA valid.
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
	// Tidak ada variabel yang terisi.
	return ""
}
