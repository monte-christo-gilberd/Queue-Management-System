package app

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"qms/internal/config"
	"qms/internal/router"
	noHTMLValidator "qms/internal/validator"
)

// NewEngine menyiapkan engine HTTP dari appRouter: mendaftarkan semua rute, validator kustom no_html_tags, dan
// trusted proxies dari cfg. Mengembalikan error bila TRUSTED_PROXIES tidak valid.
func NewEngine(cfg *config.Config, appRouter *router.AppRouter) (*gin.Engine, error) {
	// membuat engine gin (router HTTP dari framework Gin) dan mendaftarkan semua rute dari appRouter
	engine := router.SetupRouter(appRouter)

	// daftarkan validator kustom no_html_tags ke validator bawaan gin
	// RegisterValidation = mendaftarkan aturan validasi; no_html_tags menolak input berisi tag HTML
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterValidation(
			"no_html_tags",
			noHTMLValidator.NoHTMLTags,
		)
	}

	// Tanpa daftar, header X-Forwarded-For dari klien tidak dipercaya (IP klien = alamat koneksi).
	//
	// memasang trusted proxies dari config ke engine
	// trusted proxies = proxy yang header X-Forwarded-For-nya dipercaya untuk membaca IP klien asli
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {

		// jika ada error saat pemasangan, kembalikan ke pemanggil
		return nil, fmt.Errorf("invalid TRUSTED_PROXIES: %w", err)
	}

	return engine, nil
}
