package metrics

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// Templat route akun (group /api/v1/users). Harus sama dengan internal/router (dijaga tes).
const (
	routeLogin          = "/api/v1/users/login"
	routeRegister       = "/api/v1/users/register"
	routeForgotPassword = "/api/v1/users/forgot-password"
	routeResetPassword  = "/api/v1/users/reset-password"
	routeVerifyEmail    = "/api/v1/users/verify-email"
)

// Auth memegang metrik bisnis auth-service.
type Auth struct {
	*Metrics
	// login: hasil login password.
	login *prometheus.CounterVec
	// register: hasil registrasi akun.
	register *prometheus.CounterVec
	// reset: lupa dan reset password, per langkah (requested, completed) dan hasil.
	reset *prometheus.CounterVec
	// verify: hasil verifikasi email.
	verify *prometheus.CounterVec
}

// NewAuth membuat Metrics auth-service lengkap dengan metrik bisnis.
func NewAuth() *Auth {
	// registry dasar dengan runtime Go dan proses
	a := &Auth{Metrics: New("auth-service")}
	result := []string{"result"}
	a.login = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_login_total", Help: "Hasil login password: ok, atau kode galat huruf kecil (mis. invalid_credentials).",
	}, result)
	a.register = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_register_total", Help: "Hasil registrasi akun: ok, atau kode galat huruf kecil (mis. email_taken).",
	}, result)
	a.reset = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_password_reset_total", Help: "Reset password per langkah (requested = minta tautan, completed = password diganti) dan hasil.",
	}, []string{"step", "result"})
	a.verify = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "auth_email_verify_total", Help: "Hasil verifikasi email: ok, atau kode galat huruf kecil.",
	}, result)
	a.Reg.MustRegister(a.login, a.register, a.reset, a.verify)
	// nilai awal 0 untuk hasil yang pasti ada, supaya grafik dan alert tidak menunggu kejadian pertama
	a.login.WithLabelValues("ok")
	a.register.WithLabelValues("ok")
	a.reset.WithLabelValues("requested", "ok")
	a.reset.WithLabelValues("completed", "ok")
	a.verify.WithLabelValues("ok")
	return a
}

// Middleware mengembalikan middleware HTTP umum yang juga memicu hook metrik bisnis.
func (a *Auth) Middleware() gin.HandlerFunc {
	return a.HTTPMiddleware(a.hook)
}

// outcome menerjemahkan hasil request ke label result: "ok" untuk 2xx, selain itu kode galat aplikasi huruf kecil
// (himpunannya kecil dan tetap), atau "error" bila tidak ada kode.
func outcome(
	c *gin.Context,
	status int,
) string {
	// sukses
	if status >= 200 && status < 300 {
		return "ok"
	}
	// kode galat aplikasi yang dicatat respondError
	if code := c.GetString("error_code"); code != "" {
		return strings.ToLower(code)
	}
	// galat tanpa kode (mis. dari middleware atau panic)
	return "error"
}

// hook membaca hasil request untuk metrik bisnis. Tidak mengubah logika auth.
func (a *Auth) hook(
	c *gin.Context,
	route string,
	status int,
) {
	// semua rute akun memakai POST
	if c.Request.Method != http.MethodPost {
		return
	}
	switch route {
	case routeLogin:
		a.login.WithLabelValues(outcome(c, status)).Inc()
	case routeRegister:
		a.register.WithLabelValues(outcome(c, status)).Inc()
	case routeForgotPassword:
		a.reset.WithLabelValues("requested", outcome(c, status)).Inc()
	case routeResetPassword:
		a.reset.WithLabelValues("completed", outcome(c, status)).Inc()
	case routeVerifyEmail:
		a.verify.WithLabelValues(outcome(c, status)).Inc()
	}
}
