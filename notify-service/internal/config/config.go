// Package config membaca konfigurasi notify-service dari environment.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL     string
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
}

// Load gagal cepat bila ada variabel wajib yang kosong.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		VAPIDPublicKey:  os.Getenv("VAPID_PUBLIC_KEY"),
		VAPIDPrivateKey: os.Getenv("VAPID_PRIVATE_KEY"),
		VAPIDSubject:    os.Getenv("VAPID_SUBJECT"),
	}
	for name, v := range map[string]string{
		"DATABASE_URL":      cfg.DatabaseURL,
		"VAPID_PUBLIC_KEY":  cfg.VAPIDPublicKey,
		"VAPID_PRIVATE_KEY": cfg.VAPIDPrivateKey,
		"VAPID_SUBJECT":     cfg.VAPIDSubject,
	} {
		if v == "" {
			return Config{}, fmt.Errorf("missing required env var: %s", name)
		}
	}
	return cfg, nil
}
