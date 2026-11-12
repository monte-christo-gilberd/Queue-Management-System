package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"qms/internal/metrics"
)

// fakeHub memenuhi metrics.ClientCounter.
type fakeHub struct{ q, d int }

func (f fakeHub) ClientCounts() (int, int) { return f.q, f.d }

// newBackendEngine membuat engine dengan middleware metrik backend. respond mengatur status dan kode galat per route.
func newBackendEngine(t *testing.T, hub metrics.ClientCounter) (*gin.Engine, *metrics.Backend) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	b := metrics.NewBackend(hub)
	engine := gin.New()
	engine.Use(b.Middleware())
	return engine, b
}

// reply membuat handler yang membalas status dan (opsional) kode galat seperti respondError.
func reply(status int, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if code != "" {
			c.Set("error_code", code)
		}
		c.Status(status)
	}
}

func do(engine *gin.Engine, method, path string) {
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
}

// Tiket masuk dihitung dari join dan insert yang sukses saja; Next sukses dihitung sebagai dilayani.
func TestBackend_TicketCounters(t *testing.T) {
	engine, b := newBackendEngine(t, nil)
	engine.POST("/api/v1/join", reply(http.StatusOK, ""))
	engine.POST("/api/v1/queues/:queue_id/insert", reply(http.StatusOK, ""))
	engine.POST("/api/v1/queues/:queue_id/next", reply(http.StatusOK, ""))
	engine.POST("/api/v1/queues/:queue_id/skip", reply(http.StatusOK, ""))

	do(engine, http.MethodPost, "/api/v1/join")
	do(engine, http.MethodPost, "/api/v1/queues/5/insert")
	do(engine, http.MethodPost, "/api/v1/queues/5/next")
	do(engine, http.MethodPost, "/api/v1/queues/5/next")
	do(engine, http.MethodPost, "/api/v1/queues/5/skip")

	if got := counter(t, b.Registry, "tickets_joined_total", nil); got != 2 {
		t.Fatalf("tickets_joined_total = %v, diharapkan 2", got)
	}
	if got := counter(t, b.Registry, "tickets_served_total", nil); got != 2 {
		t.Fatalf("tickets_served_total = %v, diharapkan 2", got)
	}
}

// Join yang gagal tidak dihitung sebagai tiket masuk, dan EMAIL_NOT_VERIFIED dihitung per aksi.
func TestBackend_FailedJoinNotCounted_EmailRejected(t *testing.T) {
	engine, b := newBackendEngine(t, nil)
	engine.POST("/api/v1/join", reply(http.StatusForbidden, "EMAIL_NOT_VERIFIED"))
	engine.POST("/api/v1/queues", reply(http.StatusForbidden, "EMAIL_NOT_VERIFIED"))

	do(engine, http.MethodPost, "/api/v1/join")
	do(engine, http.MethodPost, "/api/v1/queues")
	do(engine, http.MethodPost, "/api/v1/queues")

	if got := counter(t, b.Registry, "tickets_joined_total", nil); got != 0 {
		t.Fatalf("join gagal tidak boleh dihitung: %v", got)
	}
	if got := counter(t, b.Registry, "email_not_verified_rejected_total", map[string]string{"action": "queue_create"}); got != 2 {
		t.Fatalf("queue_create = %v, diharapkan 2", got)
	}
	if got := counter(t, b.Registry, "email_not_verified_rejected_total", map[string]string{"action": "join"}); got != 1 {
		t.Fatalf("join = %v, diharapkan 1", got)
	}
}

// 402 SUBSCRIPTION_INACTIVE dihitung per route dari kode galat.
func TestBackend_SubscriptionInactive(t *testing.T) {
	engine, b := newBackendEngine(t, nil)
	engine.POST("/api/v1/queues/:queue_id/next", reply(http.StatusPaymentRequired, "SUBSCRIPTION_INACTIVE"))

	do(engine, http.MethodPost, "/api/v1/queues/1/next")

	if got := counter(t, b.Registry, "subscription_inactive_rejected_total", map[string]string{"route": "/api/v1/queues/:queue_id/next"}); got != 1 {
		t.Fatalf("subscription_inactive_rejected_total = %v, diharapkan 1", got)
	}
	if got := counter(t, b.Registry, "tickets_served_total", nil); got != 0 {
		t.Fatalf("Next yang ditolak tidak boleh dihitung dilayani: %v", got)
	}
}

// Hasil perpanjangan langganan: ok, email_not_verified, atau error; nilai awal 0 sudah ada.
func TestBackend_SubscriptionExtendResults(t *testing.T) {
	engine, b := newBackendEngine(t, nil)
	status, code := http.StatusOK, ""
	engine.POST("/api/v1/owners/subscription/extend", func(c *gin.Context) { reply(status, code)(c) })

	for _, r := range []string{"ok", "email_not_verified", "error"} {
		if find(t, b.Registry, "subscription_extend_total", map[string]string{"result": r}) == nil {
			t.Fatalf("deret awal result=%s tidak ada", r)
		}
	}
	do(engine, http.MethodPost, "/api/v1/owners/subscription/extend")
	status, code = http.StatusForbidden, "EMAIL_NOT_VERIFIED"
	do(engine, http.MethodPost, "/api/v1/owners/subscription/extend")
	status, code = http.StatusInternalServerError, "INTERNAL_ERROR"
	do(engine, http.MethodPost, "/api/v1/owners/subscription/extend")

	for _, r := range []string{"ok", "email_not_verified", "error"} {
		if got := counter(t, b.Registry, "subscription_extend_total", map[string]string{"result": r}); got != 1 {
			t.Fatalf("subscription_extend_total{%s} = %v, diharapkan 1", r, got)
		}
	}
	if got := counter(t, b.Registry, "email_not_verified_rejected_total", map[string]string{"action": "subscription_extend"}); got != 1 {
		t.Fatalf("subscription_extend rejected = %v, diharapkan 1", got)
	}
}

// Gauge SSE membaca jumlah klien dari hub saat di-scrape, dipisah queue dan display.
func TestBackend_SSEGauge(t *testing.T) {
	_, b := newBackendEngine(t, fakeHub{q: 3, d: 2})

	q := find(t, b.Registry, "sse_clients_active", map[string]string{"room": "queue"})
	d := find(t, b.Registry, "sse_clients_active", map[string]string{"room": "display"})
	if q == nil || q.GetGauge().GetValue() != 3 || d == nil || d.GetGauge().GetValue() != 2 {
		t.Fatalf("gauge SSE tidak sesuai: queue=%v display=%v", q, d)
	}
}
