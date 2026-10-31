package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"qms/internal/middleware"

	"github.com/gin-gonic/gin"
)

func do(t *testing.T, allowed []string, method, origin string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CORS(allowed))
	r.Any("/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest(method, "/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCORSAllowedOriginReflected(t *testing.T) {
	w := do(t, []string{"https://qms.example"}, http.MethodGet, "https://qms.example")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://qms.example" {
		t.Fatalf("got %q", got)
	}
	if w.Header().Get("Vary") != "Origin" {
		t.Fatal("missing Vary: Origin")
	}
}

func TestCORSUnknownOriginGetsNoHeaders(t *testing.T) {
	w := do(t, []string{"https://qms.example"}, http.MethodGet, "https://evil.example")
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected ACAO %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	w := do(t, []string{"https://qms.example"}, http.MethodOptions, "https://qms.example")
	if w.Code != http.StatusNoContent {
		t.Fatalf("code %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("missing allow methods")
	}
}

func TestCORSWildcardOnlyWhenConfigured(t *testing.T) {
	w := do(t, []string{"*"}, http.MethodGet, "https://any.example")
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("wildcard not applied")
	}
}
