package router

import "net/http"

// AppRouter menyimpan handler dan middleware yang akan didaftarkan oleh SetupRouter.
type AppRouter struct {
	// mailHandler: handler kirim email.
	mailHandler MailHandler
	// authMiddleware: pemeriksaan X-Service-Token untuk rute yang dilindungi.
	authMiddleware ServiceTokenMiddleware
	// systemHandler: handler health check dan 404 (opsional).
	systemHandler SystemHandler
}

// NewAppRouter membuat AppRouter dengan handler wajib: kirim email dan middleware token layanan.
// Fitur opsional dipasang setelahnya lewat metode With...
func NewAppRouter(
	mailHandler MailHandler,
	authMiddleware ServiceTokenMiddleware,
) *AppRouter {
	// isi AppRouter dengan handler dan middleware wajib
	app := &AppRouter{
		mailHandler:    mailHandler,
		authMiddleware: authMiddleware,
	}
	// kembalikan router yang siap dipasang fitur opsional
	return app
}

// WithSystem memasang rute GET /health dan balasan 404 untuk rute lain (opsional; tanpa ini keduanya tidak ada).
func (a *AppRouter) WithSystem(
	h SystemHandler,
) *AppRouter {
	// simpan handler sistem, lalu kembalikan router yang sama agar bisa dirangkai
	a.systemHandler = h
	// Kembalikan router yang sama agar pemanggilan bisa dirangkai.
	return a
}

// SetupRouter membuat http.ServeMux dan mendaftarkan semua rute sesuai handler yang sudah dipasang di router.
func SetupRouter(
	router *AppRouter,
) http.Handler {
	// router standar Go dengan pola metode dan path
	mux := http.NewServeMux()
	// rute sistem hanya didaftarkan bila dipasang
	if router.systemHandler != nil {
		// Health check: selalu 200 selama proses hidup.
		mux.HandleFunc("GET /health", router.systemHandler.Health)
	}
	// rute kirim email hanya didaftarkan bila handler ada
	if router.mailHandler != nil {
		registerMailRoutes(
			mux,
			router.mailHandler,
			router.authMiddleware,
		)
	}
	// rute lain: balas 404 dengan format error yang seragam
	if router.systemHandler != nil {
		// Pola "/" menangkap semua rute yang tidak cocok dengan rute lain.
		mux.HandleFunc("/", router.systemHandler.NotFound)
	}
	// semua rute sudah terdaftar; kembalikan handler untuk dijalankan server
	return mux
}
