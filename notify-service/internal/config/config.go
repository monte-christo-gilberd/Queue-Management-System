package config

import (
	"fmt"
	"os"
)

// Config adalah konfigurasi runtime notify-service. Semua field wajib diisi.
type Config struct {
	// DatabaseURL: URL koneksi PostgreSQL (DATABASE_URL). Dipakai untuk LISTEN dan query baca.
	DatabaseURL string
	// VAPIDPublicKey: kunci publik VAPID (VAPID_PUBLIC_KEY). Dibagikan ke browser saat berlangganan.
	VAPIDPublicKey string
	// VAPIDPrivateKey: kunci privat VAPID (VAPID_PRIVATE_KEY). Dipakai menandatangani setiap push.
	VAPIDPrivateKey string
	// VAPIDSubject: kontak pengirim push (VAPID_SUBJECT), biasanya mailto: atau URL.
	VAPIDSubject string
}

// Load gagal cepat bila ada variabel wajib yang kosong.
func Load() (Config, error) {
	// Baca semua variabel dari environment ke Config.
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    os.Getenv("VAPID_SUBJECT"),
	}
	// Periksa setiap variabel wajib. Bila ada yang kosong, tolak dengan menyebut namanya.
	for name, v := range map[string]string{
		"DATABASE_URL":      cfg.DatabaseURL,
		"VAPID_PUBLIC_KEY":  cfg.VAPIDPublicKey,
		"VAPID_PRIVATE_KEY": cfg.VAPIDPrivateKey,
		"VAPID_SUBJECT":     cfg.VAPIDSubject,
	} {
		// Nilai kosong: service tidak bisa berjalan dengan benar, jadi kembalikan error.
		if v == "" {
			return Config{}, fmt.Errorf("missing required env var: %s", name)
		}
	}
	// Semua variabel wajib terisi: kembalikan konfigurasi.
	return cfg, nil
}
