package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"qms/auth-service/internal/middleware"
	"qms/auth-service/internal/requestid"
)

// serveWithRequestID menjalankan satu request lewat middleware RequestID dan mengembalikan respons serta ID yang
// terlihat oleh handler (di context Gin dan di context request).
func serveWithRequestID(t *testing.T, header string) (*httptest.ResponseRecorder, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var ginID, ctxID string
	engine := gin.New()
	engine.Use(middleware.RequestID())
	engine.GET("/", func(c *gin.Context) {
		ginID = c.GetString("request_id")
		ctxID = requestid.From(c.Request.Context())
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set("X-Request-Id", header)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w, ginID, ctxID
}

// ID yang wajar dari nginx dipakai apa adanya dan dikirim balik di respons.
func TestRequestID_ReusesValidHeader(t *testing.T) {
	w, ginID, ctxID := serveWithRequestID(t, "9f8e7d6c5b4a-test_1.2")
	if ginID != "9f8e7d6c5b4a-test_1.2" || ctxID != ginID || w.Header().Get("X-Request-Id") != ginID {
		t.Fatalf("ID tidak dipakai: gin=%q ctx=%q header=%q", ginID, ctxID, w.Header().Get("X-Request-Id"))
	}
}

// Tanpa header dibuat ID baru 24 karakter hex.
func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	w, ginID, ctxID := serveWithRequestID(t, "")
	if len(ginID) != 24 || ctxID != ginID || w.Header().Get("X-Request-Id") != ginID {
		t.Fatalf("ID baru tidak sesuai: gin=%q ctx=%q header=%q", ginID, ctxID, w.Header().Get("X-Request-Id"))
	}
}

// ID terlalu panjang atau berisi karakter di luar daftar diganti ID baru.
func TestRequestID_ReplacesInvalidHeader(t *testing.T) {
	for _, bad := range []string{strings.Repeat("a", 129), "ada spasi", `kutip"`, "baris{json}"} {
		_, ginID, _ := serveWithRequestID(t, bad)
		if ginID == bad || len(ginID) != 24 {
			t.Fatalf("ID %q seharusnya diganti, dapat %q", bad, ginID)
		}
	}
}
