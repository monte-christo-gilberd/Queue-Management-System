package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"qms/auth-service/internal/metrics"
)

func authEngine(t *testing.T) (*gin.Engine, *metrics.Auth) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	a := metrics.NewAuth()
	engine := gin.New()
	engine.Use(a.Middleware())
	return engine, a
}

func reply(status int, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if code != "" {
			c.Set("error_code", code)
		}
		c.Status(status)
	}
}

func post(engine *gin.Engine, path string) {
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, path, nil))
}

// Hasil login dan registrasi: ok untuk 2xx, selain itu kode galat huruf kecil.
func TestAuth_LoginAndRegisterResults(t *testing.T) {
	engine, a := authEngine(t)
	status, code := http.StatusOK, ""
	h := func(c *gin.Context) { reply(status, code)(c) }
	engine.POST("/api/v1/users/login", h)
	engine.POST("/api/v1/users/register", h)

	post(engine, "/api/v1/users/login")
	status, code = http.StatusUnauthorized, "INVALID_CREDENTIALS"
	post(engine, "/api/v1/users/login")
	post(engine, "/api/v1/users/login")
	status, code = http.StatusConflict, "EMAIL_TAKEN"
	post(engine, "/api/v1/users/register")
	status, code = http.StatusInternalServerError, ""
	post(engine, "/api/v1/users/register")

	check := func(name, result string, want float64) {
		t.Helper()
		if got := counter(t, a.Registry, name, map[string]string{"result": result}); got != want {
			t.Errorf("%s{result=%s} = %v, diharapkan %v", name, result, got, want)
		}
	}
	check("auth_login_total", "ok", 1)
	check("auth_login_total", "invalid_credentials", 2)
	check("auth_register_total", "email_taken", 1)
	check("auth_register_total", "error", 1)
}

// Reset password dipisah per langkah, dan verifikasi email dihitung per hasil.
func TestAuth_ResetAndVerify(t *testing.T) {
	engine, a := authEngine(t)
	engine.POST("/api/v1/users/forgot-password", reply(http.StatusOK, ""))
	engine.POST("/api/v1/users/reset-password", reply(http.StatusBadRequest, "INVALID_TOKEN"))
	engine.POST("/api/v1/users/verify-email", reply(http.StatusOK, ""))

	post(engine, "/api/v1/users/forgot-password")
	post(engine, "/api/v1/users/reset-password")
	post(engine, "/api/v1/users/verify-email")

	if got := counter(t, a.Registry, "auth_password_reset_total", map[string]string{"step": "requested", "result": "ok"}); got != 1 {
		t.Errorf("requested ok = %v", got)
	}
	if got := counter(t, a.Registry, "auth_password_reset_total", map[string]string{"step": "completed", "result": "invalid_token"}); got != 1 {
		t.Errorf("completed invalid_token = %v", got)
	}
	if got := counter(t, a.Registry, "auth_email_verify_total", map[string]string{"result": "ok"}); got != 1 {
		t.Errorf("verify ok = %v", got)
	}
}

// Hanya metode POST yang dihitung sebagai aksi akun (GET ke route yang sama diabaikan hook).
func TestAuth_NonPostIgnored(t *testing.T) {
	engine, a := authEngine(t)
	engine.GET("/api/v1/users/login", reply(http.StatusOK, ""))

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/login", nil))

	if got := counter(t, a.Registry, "auth_login_total", map[string]string{"result": "ok"}); got != 0 {
		t.Errorf("GET tidak boleh dihitung sebagai login: %v", got)
	}
}
