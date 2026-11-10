// Package config membaca dan memvalidasi konfigurasi qr-service dari environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// DefaultDynamicTTL adalah masa berlaku token join dinamis bila tidak diatur lewat env.
const DefaultDynamicTTL = 120 * time.Second

const (
	// minDynamicTTL: batas bawah masa berlaku QR dinamis (30 detik).
	minDynamicTTL = 30 * time.Second
	// maxDynamicTTL: batas atas masa berlaku QR dinamis (10 menit).
	maxDynamicTTL = 10 * time.Minute
)

// Config adalah konfigurasi runtime qr-service. Service ini stateless: tidak ada
// database, hanya secret untuk menandatangani token.
type Config struct {
	// Port: port HTTP tempat service mendengarkan. Bawaan 8090.
	Port string
	// JoinSecret: kunci HMAC untuk menandatangani token join. Minimal 32 byte.
	JoinSecret string
	// DynamicTTL: masa berlaku token join dinamis.
	DynamicTTL time.Duration
	// ServiceToken: nilai X-Service-Token yang harus dibawa setiap request internal dari backend.
	ServiceToken string
	// LogLevel: level log (debug, info, warn, error). Bawaan info.
	LogLevel string
}

// Load membaca environment. Variabel wajib yang kosong menghasilkan error agar
// container gagal start cepat daripada melayani token yang tidak bisa dipakai.
func Load() (Config, error) {
	// Isi nilai dari environment, dengan bawaan untuk PORT dan LOG_LEVEL.
	cfg := Config{
		Port:         envOr("PORT", "8090"),
		JoinSecret:   os.Getenv("JOIN_LINK_SECRET"),
		ServiceToken: os.Getenv("QR_SERVICE_TOKEN"),
		LogLevel:     envOr("LOG_LEVEL", "info"),
	}

	// JOIN_LINK_SECRET kosong: tanpa secret, token tidak bisa ditandatangani, jadi tolak.
	if cfg.JoinSecret == "" {
		return Config{}, fmt.Errorf("missing required env var: JOIN_LINK_SECRET")
	}
	// Secret kurang dari 32 byte terlalu mudah ditebak, jadi tolak.
	if len(cfg.JoinSecret) < 32 {
		return Config{}, fmt.Errorf("JOIN_LINK_SECRET must be at least 32 bytes")
	}
	// QR_SERVICE_TOKEN kosong: endpoint internal tidak bisa diamankan, jadi tolak.
	if cfg.ServiceToken == "" {
		return Config{}, fmt.Errorf("missing required env var: QR_SERVICE_TOKEN")
	}

	// Mulai dari bawaan, lalu timpa bila QR_DYNAMIC_TTL_SECONDS diisi.
	ttl := DefaultDynamicTTL
	// Variabel TTL diisi: ubah dari teks ke detik. Nilai yang tidak bisa diubah menjadi angka ditolak.
	if raw := os.Getenv("QR_DYNAMIC_TTL_SECONDS"); raw != "" {
		seconds, err := strconv.Atoi(raw)
		// Teks bukan angka: tolak dengan membungkus error asli.
		if err != nil {
			return Config{}, fmt.Errorf("invalid QR_DYNAMIC_TTL_SECONDS: %w", err)
		}
		// Ubah jumlah detik menjadi durasi.
		ttl = time.Duration(seconds) * time.Second
	}
	// TTL di luar rentang yang diizinkan: tolak dengan pesan yang menyebut batasnya.
	if ttl < minDynamicTTL || ttl > maxDynamicTTL {
		return Config{}, fmt.Errorf(
			"QR_DYNAMIC_TTL_SECONDS must be between %d and %d",
			int(minDynamicTTL.Seconds()), int(maxDynamicTTL.Seconds()),
		)
	}
	// TTL lolos validasi: simpan ke konfigurasi.
	cfg.DynamicTTL = ttl

	return cfg, nil
}

// envOr mengembalikan isi environment variable key, atau fallback bila variabel itu kosong.
func envOr(key, fallback string) string {
	// Variabel ada dan tidak kosong: pakai nilainya.
	if v := os.Getenv(key); v != "" {
		return v
	}
	// Variabel tidak ada atau kosong: pakai nilai bawaan.
	return fallback
}
