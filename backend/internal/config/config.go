package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config menyimpan konfigurasi aplikasi yang dibaca dari environment variable (nilai pengaturan dari sistem
// operasi atau container). File .env juga dibaca lewat godotenv, library Go untuk memuat file .env ke environment variable.
//
// Dibuat oleh Load() dan dipakai di main() (cmd/server/main.go) untuk merangkai database, service, klien
// microservice (qrclient dan mailclient), dan router. Load() juga memvalidasi isinya.
//
// Wajib diisi: DATABASE_URL, JWT_SECRET, ALLOWED_ORIGINS, PUBLIC_APP_URL, QR_SERVICE_URL, QR_SERVICE_TOKEN.
// Opsional: MAIL_SERVICE_URL dan MAIL_SERVICE_TOKEN (harus keduanya atau tidak sama sekali), VAPID_PUBLIC_KEY,
// APP_ENV, TRUSTED_PROXIES, dan PORT (bawaan 8080).
// Opsional dengan nilai bawaan dalam detik: QR_HTTP_TIMEOUT (1) dan SHUTDOWN_TIMEOUT (8).
type Config struct {
	// Env: APP_ENV. Bila "production", validateProduction() menolak secret lemah (terlalu pendek atau nilai contoh) dan origin/URL yang tidak https.
	Env string
	// URL koneksi ke PostgreSQL (DATABASE_URL), dipakai database.Connect.
	DatabaseURL string
	// kunci rahasia untuk menandatangani token login JWT (JWT_SECRET).
	// JSON Web Token: token login yang diterbitkan server
	JWTSecret string
	// port HTTP tempat server listen (PORT). Bila kosong, default 8080.
	Port string
	// origin yang boleh memanggil API dari browser.
	// Origin = skema + host + port, dipisah koma (ALLOWED_ORIGINS). Contoh: https://app.example.com
	// CORS: aturan browser yang membatasi akses lintas domain
	AllowedOrigins []string
	// TrustedProxies: IP atau CIDR (rentang alamat, misalnya 10.0.0.0/8) dari reverse proxy yang boleh menentukan IP klien
	// lewat header X-Forwarded-For (header yang ditambahkan proxy seperti nginx untuk mencatat IP asal). Kosong = tidak ada proxy yang dipercaya.
	TrustedProxies []string

	// URL publik aplikasi frontend (PUBLIC_APP_URL). Dipakai untuk membuat tautan join QR dan tautan reset password.
	PublicAppURL string
	// URL dasar qr-service (QR_SERVICE_URL). qr-service adalah microservice terpisah yang membuat dan memverifikasi
	// token join QR. Backend memanggil endpoint internalnya, /internal/v1/*, yang tidak dibuka lewat nginx.
	QRServiceURL string
	// token autentikasi antarservice dari backend ke qr-service (QR_SERVICE_TOKEN), dikirim sebagai header
	// X-Service-Token. Ini bukan token login user.
	QRServiceToken string
	// batas waktu tiap panggilan ke qr-service dalam detik (QR_HTTP_TIMEOUT). Bila kosong atau nilai <= 0, 1 detik.
	// Nilai yang bukan angka membuat Load() gagal.
	QRHTTPTimeout time.Duration
	// batas waktu menunggu request yang sedang berjalan selesai saat server berhenti (SHUTDOWN_TIMEOUT, detik).
	// Bila kosong atau nilai <= 0, 8 detik. Harus lebih kecil dari stop_grace_period backend di compose.
	// Nilai yang bukan angka membuat Load() gagal.
	ShutdownTimeout time.Duration

	// mail-service (reset password): microservice terpisah yang mengirim email. Opsional sebagai pasangan: keduanya diisi atau keduanya kosong. Kosong = fitur
	// lupa password tidak dipasang (rute 404).
	MailServiceURL string
	// token autentikasi backend ke mail-service (MAIL_SERVICE_TOKEN).
	MailServiceToken string

	// kunci publik VAPID. Opsional; kosong = fitur push mati.
	// Voluntary Application Server Identification: pasangan kunci untuk Web Push, yaitu notifikasi
	// ke browser saat tab tertutup
	VAPIDPublicKey string
}

// Load membaca environment variable (dari file .env bila ada, atau dari container), memvalidasi, lalu mengembalikan Config.
// Dipanggil sekali di main(). Bila mengembalikan error, server tidak boleh dijalankan.
func Load() (Config, error) {

	// memuat file .env ke environment variable. Variabel yang sudah ada tidak ditimpa. Error-nya diabaikan
	// karena container biasanya tidak punya file .env.
	_ = godotenv.Load()

	// mengisi Config dari environment variable. ALLOWED_ORIGINS dan TRUSTED_PROXIES memakai parser daftar koma yang sama.
	// URL publik dan URL qr-service dibuang '/' di akhirnya agar tidak menghasilkan '//' saat digabung dengan path.
	cfg := Config{
		Env:            strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		Port:           os.Getenv("PORT"),
		AllowedOrigins: (parseOrigins(os.Getenv("ALLOWED_ORIGINS"))),
		TrustedProxies: parseOrigins(os.Getenv("TRUSTED_PROXIES")),

		PublicAppURL:   strings.TrimRight(os.Getenv("PUBLIC_APP_URL"), "/"),
		QRServiceURL:   strings.TrimRight(os.Getenv("QR_SERVICE_URL"), "/"),
		QRServiceToken: os.Getenv("QR_SERVICE_TOKEN"),
		VAPIDPublicKey: os.Getenv("VAPID_PUBLIC_KEY"),

		MailServiceURL:   strings.TrimRight(os.Getenv("MAIL_SERVICE_URL"), "/"),
		MailServiceToken: os.Getenv("MAIL_SERVICE_TOKEN"),
	}

	// MAIL_SERVICE_URL dan MAIL_SERVICE_TOKEN harus sama-sama diisi atau sama-sama kosong. Hanya salah satu = error.
	if (cfg.MailServiceURL == "") != (cfg.MailServiceToken == "") {
		return Config{}, fmt.Errorf("MAIL_SERVICE_URL and MAIL_SERVICE_TOKEN must be set together")
	}

	// pengecekan apakah cfg.DatabaseURL ada
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("missing required env var: DATABASE_URL")
	}

	// pengecekan apakah cfg.JWTSecret ada
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("missing required env var: JWT_SECRET")
	}

	// ALLOWED_ORIGINS wajib ada. parseOrigins mengembalikan nil bila string kosong.
	// Catatan: isi yang hanya berupa koma atau spasi
	// tidak dianggap kosong, karena parseOrigins mengembalikan slice kosong, bukan nil.
	if cfg.AllowedOrigins == nil {
		return Config{}, fmt.Errorf("missing required env var: ALLOWED_ORIGINS")
	}

	// PORT kosong = 8080
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	// pengecekan apakah cfg.PublicAppURL ada
	if cfg.PublicAppURL == "" {
		return Config{}, fmt.Errorf("missing required env var: PUBLIC_APP_URL")
	}

	// pengecekan apakah cfg.QRServiceURL ada
	if cfg.QRServiceURL == "" {
		return Config{}, fmt.Errorf("missing required env var: QR_SERVICE_URL")
	}

	// pengecekan apakah cfg.QRServiceToken ada
	if cfg.QRServiceToken == "" {
		return Config{}, fmt.Errorf("missing required env var: QR_SERVICE_TOKEN")
	}

	// QR_HTTP_TIMEOUT dalam detik. Kosong = 1 detik. Bukan angka = error. Nilai <= 0 = 1 detik.
	timeout := time.Second
	if raw := os.Getenv("QR_HTTP_TIMEOUT"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid QR_HTTP_TIMEOUT: %w", err)
		}
		timeout = time.Duration(seconds) * time.Second
	}
	if timeout <= 0 {
		timeout = time.Second
	}
	cfg.QRHTTPTimeout = timeout

	// SHUTDOWN_TIMEOUT dalam detik. Kosong = 8 detik. Bukan angka = error. Nilai <= 0 = 8 detik.
	shutdownTimeout := 8 * time.Second
	if raw := os.Getenv("SHUTDOWN_TIMEOUT"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SHUTDOWN_TIMEOUT: %w", err)
		}
		shutdownTimeout = time.Duration(seconds) * time.Second
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = 8 * time.Second
	}
	cfg.ShutdownTimeout = shutdownTimeout

	// pengecekan apakah cfg.Env adalah "production"
	if cfg.Env == "production" {

		// validasi khusus untuk production
		if err := cfg.validateProduction(); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

// nilai contoh dari .env.example (change-me-...) dan kata umum (secret, password) yang tidak boleh dipakai sebagai secret atau token saat production
var weakSecrets = map[string]bool{
	"change-me-jwt-secret":                           true,
	"change-me-service-token-at-least-32-bytes":      true,
	"change-me-mail-service-token-at-least-32-bytes": true,
	"secret":   true,
	"password": true,
}

// validateProduction menolak konfigurasi yang tidak aman saat APP_ENV=production.
func (c Config) validateProduction() error {

	// pengecekan apakah c.JWTSecret lebih pendek dari 32 atau ada di weakSecrets
	if len(c.JWTSecret) < 32 || weakSecrets[c.JWTSecret] {
		return fmt.Errorf("production: JWT_SECRET must be at least 32 chars and not a default value")
	}
	// pengecekan apakah c.QRServiceToken lebih pendek dari 32 atau ada di weakSecrets
	if len(c.QRServiceToken) < 32 || weakSecrets[c.QRServiceToken] {
		return fmt.Errorf("production: QR_SERVICE_TOKEN must be at least 32 chars and not a default value")
	}
	// pengecekan apakah c.MailServiceToken ada dan lebih pendek dari 32 atau ada di weakSecrets
	if c.MailServiceToken != "" && (len(c.MailServiceToken) < 32 || weakSecrets[c.MailServiceToken]) {
		return fmt.Errorf("production: MAIL_SERVICE_TOKEN must be at least 32 chars and not a default value")
	}
	for _, o := range c.AllowedOrigins {
		// menolak wildcard * karena mengizinkan origin mana pun
		if o == "*" {
			return fmt.Errorf("production: ALLOWED_ORIGINS must not contain *")
		}
		// menolak origin yang tidak memakai https
		if !strings.HasPrefix(o, "https://") {
			return fmt.Errorf("production: ALLOWED_ORIGINS must use https (got %q)", o)
		}
	}
	// PUBLIC_APP_URL harus memakai https
	if !strings.HasPrefix(c.PublicAppURL, "https://") {
		return fmt.Errorf("production: PUBLIC_APP_URL must use https")
	}
	// PUBLIC_APP_URL harus punya skema http/https dan host
	if err := validatePublicURL(c.PublicAppURL); err != nil {
		return fmt.Errorf("production: PUBLIC_APP_URL: %w", err)
	}
	return nil
}

// parseOrigins memecah daftar yang dipisah koma menjadi slice, membuang spasi dan entri kosong.
// Dipakai untuk ALLOWED_ORIGINS dan TRUSTED_PROXIES.
func parseOrigins(raw string) []string {
	if raw == "" {
		return nil
	}
	// pisahkan string raw berdasarkan ",", simpan dalam bentuk slice
	parts := strings.Split(raw, ",")
	// buat slice string baru dengan size panjang slice parts
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		// trim leading dan trailing whitespace
		if p = strings.TrimSpace(p); p != "" {
			// masukkan string yang sudah di trim ke slice out
			out = append(out, p)
		}
	}
	return out
}

// validatePublicURL memastikan raw adalah URL absolut dengan skema http atau https dan host. Dipakai untuk PUBLIC_APP_URL saat production.
func validatePublicURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid public URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("public URL must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("public URL must include a host")
	}
	return nil
}
