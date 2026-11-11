package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qms/qr-service/internal/middleware"

	"github.com/gin-gonic/gin"
)

// newEngine membuat engine dengan satu rute POST /x yang dibungkus middleware yang diuji.
func newEngine(mw ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(mw...)
	engine.POST("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return engine
}

func TestServiceToken(t *testing.T) {
	engine := newEngine(middleware.ServiceToken("secret"))

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"UNAUTHORIZED"`) {
		t.Fatalf("expected 401 UNAUTHORIZED, got %d %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Service-Token", "secret")
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestBodyLimit(t *testing.T) {
	engine := newEngine(middleware.BodyLimit(10))

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("a", 11))))
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), `"BODY_TOO_LARGE"`) {
		t.Fatalf("expected 413 BODY_TOO_LARGE, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestRequestID(t *testing.T) {
	engine := newEngine(middleware.RequestID())

	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Request-ID", "abc")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got != "abc" {
		t.Fatalf("expected forwarded id, got %q", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Request-ID", strings.Repeat("a", 129))
	rec = httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got == "" || len(got) > 128 {
		t.Fatalf("expected regenerated id, got %q", got)
	}
}

func TestNoStore(t *testing.T) {
	engine := newEngine(middleware.NoStore())

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("missing no-store headers: %v", rec.Header())
	}
}
