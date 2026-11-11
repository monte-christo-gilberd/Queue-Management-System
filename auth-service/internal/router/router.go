package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewAppRouter membuat AppRouter dengan handler wajib: registrasi, login, dan middleware login.
// Fitur opsional dipasang setelahnya lewat metode With...
func NewAppRouter(
	authHandler AuthHandler,
	authMiddleware gin.HandlerFunc,
	trustedProxies []string,
) *AppRouter {
	// isi AppRouter dengan handler dan middleware wajib
	app := &AppRouter{
		authHandler:    authHandler,
		authMiddleware: authMiddleware,
		trustedProxies: trustedProxies,
	}
	// kembalikan router yang siap dipasang fitur opsional
	return app
}

// WithEmail memasang rute reset password dan verifikasi email (opsional; tanpa ini rutenya tidak ada).
func (a *AppRouter) WithEmail(h EmailHandler) *AppRouter {
	// simpan handler email, lalu kembalikan router yang sama agar bisa dirangkai
	a.emailHandler = h
	return a
}

// WithGoogle memasang rute login Google (opsional; tanpa ini rutenya tidak ada).
func (a *AppRouter) WithGoogle(h GoogleHandler) *AppRouter {
	// simpan handler Google, lalu kembalikan router yang sama
	a.googleHandler = h
	return a
}

// SetupRouter membuat gin.Engine dan mendaftarkan semua rute sesuai handler yang sudah dipasang di router.
func SetupRouter(
	router *AppRouter,
) *gin.Engine {
	// gin.New tanpa logger bawaan: log akses dicatat nginx; Recovery mengubah panic menjadi 500
	engine := gin.New()
	// pasang Recovery agar panic di handler menjadi 500, bukan proses mati
	engine.Use(gin.Recovery())
	// hanya proxy ini yang boleh menentukan IP klien lewat X-Forwarded-For (dipakai pembatas per akun)
	_ = engine.SetTrustedProxies(router.trustedProxies)

	// Liveness: tidak menyentuh database, sehingga gangguan DB tidak membuat layanan dianggap mati.
	// rute /health langsung membalas status ok
	engine.GET("/health",
		func(c *gin.Context) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status": "ok",
				},
			)
		},
	)

	// semua rute akun berada di bawah /api/v1/users
	users := engine.Group("/api/v1/users")
	// register dan login selalu ada
	registerAuthRoutes(
		users,
		router.authHandler,
	)
	// login Google hanya dipasang bila handler Google ada; selain itu rutenya tidak ada (404)
	if router.googleHandler != nil {
		registerGoogleRoutes(
			users,
			router.googleHandler,
		)
	}
	// reset password dan verifikasi email hanya dipasang bila handler email ada; selain itu 404
	if router.emailHandler != nil {
		registerEmailRoutes(
			users,
			router.emailHandler,
		)
		// kirim ulang verifikasi butuh login: hanya rute ini di grup "me" (profil dan lainnya tetap di backend)
		// grup "me" membuat grup rute terpisah yang diberi middleware login
		me := engine.Group("/api/v1/users/me")
		// middleware login memastikan hanya user yang login bisa memanggil rute ini
		me.Use(router.authMiddleware)
		// daftarkan rute kirim ulang verifikasi di grup yang sudah diberi middleware
		registerMeRoutes(
			me,
			router.emailHandler,
		)
	}

	// semua rute sudah terdaftar; kembalikan engine untuk dijalankan server
	return engine
}
