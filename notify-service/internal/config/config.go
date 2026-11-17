package config

import (
	"fmt"
	"os"
)

// Config adalah konfigurasi runtime notify-service. Semua field wajib diisi, kecuali FCM (opsional).
type Config struct {
	// DatabaseURL: URL koneksi PostgreSQL (DATABASE_URL). Dipakai untuk LISTEN dan query baca.
	DatabaseURL string
	// VAPIDPublicKey: kunci publik VAPID (VAPID_PUBLIC_KEY). Dibagikan ke browser saat berlangganan.
	VAPIDPublicKey string
	// VAPIDPrivateKey: kunci privat VAPID (VAPID_PRIVATE_KEY). Dipakai menandatangani setiap push.
	VAPIDPrivateKey string
	// VAPIDSubject: kontak pengirim push (VAPID_SUBJECT), biasanya mailto: atau URL.
	VAPIDSubject string
	// FCMProjectID: project Firebase (FCM_PROJECT_ID). Opsional; kosong diisi dari project_id di file kredensial.
	FCMProjectID string
	// FCMCredentialsFile: path file JSON service account (FCM_CREDENTIALS_FILE). Opsional; kosong = push ke
	// aplikasi Android (FCM) mati, Web Push tetap jalan.
	FCMCredentialsFile string
}

// Load gagal cepat bila ada variabel wajib yang kosong.
func Load() (Config, error) {
	// Baca semua variabel dari environment ke Config.
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    os.Getenv("VAPID_SUBJECT"),
		// FCM opsional, jadi tidak ikut diperiksa di bawah.
		FCMProjectID:       os.Getenv("FCM_PROJECT_ID"),
		FCMCredentialsFile: os.Getenv("FCM_CREDENTIALS_FILE"),
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
