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
)

// MinSecretLen: panjang minimum JWT_LEGACY_HS256_SECRET di produksi bila diisi (sama dengan backend).
const MinSecretLen = 32

// Config adalah konfigurasi runtime payment-service.
type Config struct {
	// Port: port HTTP tempat layanan mendengarkan. Bawaan 8093.
	Port string
	// DatabaseURL: URL PostgreSQL (DATABASE_URL). Wajib.
	DatabaseURL string
	// JWTPublicKey: kunci publik RSA (JWT_PUBLIC_KEY_FILE) untuk memverifikasi token login RS256 yang diterbitkan
	// auth-service. Verifikasi dilakukan lokal, jadi pembayaran tetap bisa dimulai saat auth-service mati. Wajib.
	JWTPublicKey *rsa.PublicKey
	// JWTLegacySecret: secret HS256 lama (JWT_LEGACY_HS256_SECRET), hanya selama masa transisi agar token yang sudah
	// terbit tetap sah. Kosong = token HS256 ditolak.
	JWTLegacySecret string
	// PublicOrigin: origin publik app, tujuan kembali dari Stripe Checkout (QMS_PUBLIC_ORIGIN). Tanpa garis miring di akhir.
	PublicOrigin string
	// Production: true bila APP_ENV=production. Mengaktifkan pemeriksaan yang lebih ketat.
	Production bool

	// Stripe. Kosong/placeholder = pembayaran dinonaktifkan: service tetap hidup (health OK) dan checkout
	// menjawab 503, supaya salah konfigurasi di sini tidak pernah menjatuhkan bagian lain dari stack.

	// StripeKey: kunci API Stripe (STRIPE_API_KEY).
	StripeKey string
	// StripeBaseURL: alamat API Stripe (STRIPE_API_BASE_URL). Kosong = api.stripe.com; dev: emulator.
	StripeBaseURL string
	// WebhookSecret: secret untuk memverifikasi tanda tangan webhook (STRIPE_WEBHOOK_SECRET).
	WebhookSecret string
	// PriceID: id harga Stripe (STRIPE_PRICE_ID). Opsional; kosong = harga inline dari Amount dan Currency.
	PriceID string

	// DevSimulator: pembayaran PALSU untuk dev. Checkout langsung "berhasil" tanpa Stripe lewat jalur perpanjangan
	// yang sama. Ditolak di produksi dan bila kunci Stripe juga terisi.
	DevSimulator bool

	// Amount: harga satu bulan, satuan utama mata uang (mis. 50000 = Rp50.000).
	Amount int64
	// Currency: kode mata uang ISO 4217 dalam huruf kecil (mis. idr).
	Currency string
}

// StripeConfigured melaporkan apakah kunci dan secret webhook terisi dengan nilai sungguhan, bukan contoh.
func (c Config) StripeConfigured() bool {
	// Kedua nilai harus sungguhan agar Stripe dianggap siap dipakai.
	return usable(c.StripeKey) && usable(c.WebhookSecret)
}

// usable melaporkan apakah nilai terisi dan bukan placeholder dari .env.example.
func usable(v string) bool {
	// Buang spasi di tepi, lalu kosongkan bila hasilnya kosong.
	v = strings.TrimSpace(v)
	// Nilai kosong: tidak bisa dipakai.
	if v == "" {
		return false
	}
	// Nilai contoh dimulai dengan "ganti" atau "change-me" (tanpa memperhatikan huruf besar): bukan nilai sungguhan.
	l := strings.ToLower(v)
	return !strings.HasPrefix(l, "ganti") && !strings.HasPrefix(l, "change-me")
}

// firstEnv mengembalikan nilai variabel pertama yang tidak kosong.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// Load membaca environment dan memeriksa nilainya. Galat fatal hanya untuk hal yang membuat service tidak bisa berfungsi
// sama sekali. Kunci Stripe yang kosong BUKAN galat fatal; lihat StripeConfigured.
func Load() (Config, error) {
	// Baca semua variabel dari environment, dengan nilai bawaan untuk Amount dan Currency.
	cfg := Config{
		// PAYMENT_* didahulukan (dipakai file .env root); PORT dan DATABASE_URL tetap berlaku di container.
		Port:            strings.TrimSpace(firstEnv("PAYMENT_PORT", "PORT")),
		DatabaseURL:     strings.TrimSpace(firstEnv("PAYMENT_DATABASE_URL", "DATABASE_URL")),
		JWTLegacySecret: os.Getenv("JWT_LEGACY_HS256_SECRET"),
		PublicOrigin:    strings.TrimRight(strings.TrimSpace(os.Getenv("QMS_PUBLIC_ORIGIN")), "/"),
		Production:      strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production"),
		StripeKey:       strings.TrimSpace(os.Getenv("STRIPE_API_KEY")),
		StripeBaseURL:   strings.TrimSpace(os.Getenv("STRIPE_API_BASE_URL")),
		WebhookSecret:   strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")),
		PriceID:         strings.TrimSpace(os.Getenv("STRIPE_PRICE_ID")),
		Amount:          50000,
		Currency:        "idr",
	}
	// DEV_PAYMENT_SIMULATOR bernilai true bila berisi "true" atau "1".
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("DEV_PAYMENT_SIMULATOR"))); v == "true" || v == "1" {
		cfg.DevSimulator = true
	}
	// PORT kosong: pakai bawaan 8093.
	if cfg.Port == "" {
		cfg.Port = "8093"
	}
	// DATABASE_URL wajib: tanpanya tidak ada yang bisa disimpan.
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL must be set")
	}
	// JWT_PUBLIC_KEY_FILE wajib: tanpanya login tidak bisa diverifikasi.
	keyPath := strings.TrimSpace(os.Getenv("JWT_PUBLIC_KEY_FILE"))
	if keyPath == "" {
		return Config{}, fmt.Errorf("JWT_PUBLIC_KEY_FILE must be set")
	}
	pub, err := loadPublicKey(keyPath)
	if err != nil {
		return Config{}, fmt.Errorf("JWT_PUBLIC_KEY_FILE: %w", err)
	}
	cfg.JWTPublicKey = pub
	// Produksi: secret transisi, bila diisi, harus cukup panjang.
	if cfg.Production && cfg.JWTLegacySecret != "" && len(cfg.JWTLegacySecret) < MinSecretLen {
		return Config{}, fmt.Errorf("JWT_LEGACY_HS256_SECRET must be at least %d characters when APP_ENV=production", MinSecretLen)
	}
	// Periksa origin publik: harus URL absolut dengan skema http atau https.
	u, err := url.Parse(cfg.PublicOrigin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, fmt.Errorf("QMS_PUBLIC_ORIGIN must be an absolute http(s) URL, got %q", cfg.PublicOrigin)
	}
	// Produksi: origin wajib https.
	if cfg.Production && u.Scheme != "https" {
		return Config{}, fmt.Errorf("QMS_PUBLIC_ORIGIN must be https when APP_ENV=production, got %q", cfg.PublicOrigin)
	}
	// Simulator tidak boleh aktif di produksi karena memalsukan pembayaran.
	if cfg.DevSimulator && cfg.Production {
		return Config{}, fmt.Errorf("DEV_PAYMENT_SIMULATOR must not be enabled when APP_ENV=production (it fakes payments)")
	}
	// Simulator dan Stripe tidak boleh aktif bersamaan: pilih salah satu.
	if cfg.DevSimulator && cfg.StripeConfigured() {
		return Config{}, fmt.Errorf("DEV_PAYMENT_SIMULATOR and Stripe keys are mutually exclusive: unset one")
	}
	// Alamat API Stripe diisi: harus URL absolut http atau https.
	if cfg.StripeBaseURL != "" {
		b, err := url.Parse(cfg.StripeBaseURL)
		if err != nil || (b.Scheme != "http" && b.Scheme != "https") || b.Host == "" {
			return Config{}, fmt.Errorf("STRIPE_API_BASE_URL must be an absolute http(s) URL, got %q", cfg.StripeBaseURL)
		}
	}
	// SUBSCRIPTION_PRICE_AMOUNT diisi: harus bilangan bulat positif hingga 1 miliar.
	if v := strings.TrimSpace(os.Getenv("SUBSCRIPTION_PRICE_AMOUNT")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 || n > 1_000_000_000 {
			return Config{}, fmt.Errorf("SUBSCRIPTION_PRICE_AMOUNT must be a positive integer, got %q", v)
		}
		cfg.Amount = n
	}
	// SUBSCRIPTION_CURRENCY diisi: harus kode ISO 4217 tiga huruf.
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("SUBSCRIPTION_CURRENCY"))); v != "" {
		if len(v) != 3 {
			return Config{}, fmt.Errorf("SUBSCRIPTION_CURRENCY must be a 3-letter ISO code, got %q", v)
		}
		cfg.Currency = v
	}
	return cfg, nil
}

// loadPublicKey membaca kunci publik RSA dari berkas PEM (format PKIX, "PUBLIC KEY").
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	// Baca isi berkas kunci.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Decode blok PEM.
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("not a PEM file")
	}
	// Ubah ke kunci publik; harus RSA.
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}
	return rsaKey, nil
}
