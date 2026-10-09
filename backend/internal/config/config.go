package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	JWTSecret      string
	Port           string
	AllowedOrigins []string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		Port:           os.Getenv("PORT"),
		AllowedOrigins: (parseOrigins(os.Getenv("ALLOWED_ORIGINS"))),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("missing required env var: DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("missing required env var: JWT_SECRET")
	}
	if cfg.AllowedOrigins == nil {
		return Config{}, fmt.Errorf("missing required env var: ALLOWED_ORIGINS")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	return cfg, nil
}

func parseOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			out = append(out, o)
		}
	}
	return out
}
