package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"qms/internal/metrics"
)

// newEngine membuat engine dengan middleware metrik dan Metrics-nya, tanpa hook bisnis.
func newEngine(t *testing.T) (*gin.Engine, *metrics.Metrics) {
	t.Helper()
	// mode test mematikan log debug Gin
	gin.SetMode(gin.TestMode)
	m := metrics.New("test-svc")
	engine := gin.New()
	engine.Use(m.HTTPMiddleware(nil))
	return engine, m
}

// Label route memakai templat dan tidak pernah memuat nilai parameter (token) atau query string.
func TestHTTPMiddleware_RouteTemplateHidesSecrets(t *testing.T) {
	engine, m := newEngine(t)
	engine.GET("/display/:token", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// kirim request yang URL-nya memuat token dan query rahasia
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/display/SECRET-TOKEN?token=QUERY-SECRET", nil))

	// counter harus naik di deret berlabel templat route
	labels := map[string]string{"route": "/display/:token", "method": "GET", "status": "200", "service": "test-svc"}
	if got := counter(t, m.Registry, "http_requests_total", labels); got != 1 {
		t.Fatalf("http_requests_total = %v, diharapkan 1", got)
	}
	// tidak ada label mana pun yang boleh memuat nilai rahasia
	for _, v := range allLabelValues(t, m.Registry) {
		if strings.Contains(v, "SECRET") {
			t.Fatalf("nilai label memuat rahasia: %q", v)
		}
	}
	// histogram durasi juga harus tercatat untuk route yang sama
	if find(t, m.Registry, "http_request_duration_seconds", map[string]string{"route": "/display/:token"}) == nil {
		t.Fatal("histogram durasi tidak tercatat")
	}
}

// Rute yang tidak ada dicatat "unmatched", bukan path mentah (yang bisa memuat token atau memicu ledakan label).
func TestHTTPMiddleware_UnmatchedRoute(t *testing.T) {
	engine, m := newEngine(t)

	// request ke path yang tidak terdaftar
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/tidak/ada/TOKEN-XYZ", nil))

	// harus tercatat sebagai unmatched 404
	if got := counter(t, m.Registry, "http_requests_total", map[string]string{"route": "unmatched", "status": "404"}); got != 1 {
		t.Fatalf("unmatched 404 = %v, diharapkan 1", got)
	}
	// path mentah tidak boleh muncul di label
	for _, v := range allLabelValues(t, m.Registry) {
		if strings.Contains(v, "TOKEN-XYZ") {
			t.Fatalf("path mentah bocor ke label: %q", v)
		}
	}
}

// Rute SSE dihitung di total request tetapi tidak masuk histogram durasi (koneksinya berjam-jam, merusak p95).
func TestHTTPMiddleware_SSEExcludedFromDuration(t *testing.T) {
	engine, m := newEngine(t)
	engine.GET("/queues/:id/event", func(c *gin.Context) { c.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/queues/1/event", nil))

	// tetap terhitung sebagai request
	if got := counter(t, m.Registry, "http_requests_total", map[string]string{"route": "/queues/:id/event"}); got != 1 {
		t.Fatalf("request SSE tidak dihitung: %v", got)
	}
	// tetapi tidak boleh punya deret di histogram durasi
	if find(t, m.Registry, "http_request_duration_seconds", map[string]string{"route": "/queues/:id/event"}) != nil {
		t.Fatal("rute SSE tidak boleh masuk histogram durasi")
	}
}

// Panic yang ditangkap Recovery (dipasang setelah metrik) terhitung sebagai status 500.
func TestHTTPMiddleware_PanicCountsAs500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := metrics.New("test-svc")
	engine := gin.New()
	// urutan penting: metrik di luar Recovery, supaya status 500 hasil Recovery terlihat metrik
	engine.Use(m.HTTPMiddleware(nil), gin.Recovery())
	engine.GET("/boom", func(c *gin.Context) { panic("x") })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/boom", nil))

	if got := counter(t, m.Registry, "http_requests_total", map[string]string{"route": "/boom", "status": "500"}); got != 1 {
		t.Fatalf("panic tidak terhitung 500: %v", got)
	}
}

// Hook menerima templat route, status, dan konteks setelah request selesai.
func TestHTTPMiddleware_HookReceivesRouteAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := metrics.New("test-svc")
	// variabel penangkap nilai yang diterima hook
	var gotRoute string
	var gotStatus int
	engine := gin.New()
	engine.Use(m.HTTPMiddleware(func(c *gin.Context, route string, status int) { gotRoute, gotStatus = route, status }))
	engine.GET("/x/:id", func(c *gin.Context) { c.Status(http.StatusTeapot) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x/9", nil))

	// hook harus menerima templat (bukan /x/9) dan status yang ditulis handler
	if gotRoute != "/x/:id" || gotStatus != http.StatusTeapot {
		t.Fatalf("hook menerima route=%q status=%d", gotRoute, gotStatus)
	}
}
