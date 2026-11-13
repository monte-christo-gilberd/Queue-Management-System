package router_test

import (
	"net/http"
	"testing"

	"qms/internal/router"

	"github.com/gin-gonic/gin"
)

// Metrik bisnis backend membaca templat route tertentu. Tes ini menjaga templat itu tetap terdaftar di router,
// supaya perubahan path tidak membuat metrik diam-diam berhenti naik. Daftar harus sama dengan konstanta di
// internal/metrics/backend.go.
func TestMetricsBusinessRoutesStillRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// router dengan handler kosong (embedding antarmuka nil cukup untuk mendaftarkan rute, tidak dipanggil)
	app := router.NewAppRouter(struct{ router.TicketHandler }{}, struct{ router.UserHandler }{}, struct{ router.QueueHandler }{},
		func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		stubSSE{}, nil).
		WithSubscriptionCheckout(struct {
			router.SubscriptionCheckoutHandler
		}{})
	engine := router.SetupRouter(app)

	// kumpulkan semua rute terdaftar sebagai "METHOD /path"
	registered := map[string]bool{}
	for _, r := range engine.Routes() {
		registered[r.Method+" "+r.Path] = true
	}
	// setiap templat yang dibaca hook metrik harus ada
	for _, want := range []string{
		"POST /api/v1/join",
		"POST /api/v1/queues",
		"POST /api/v1/queues/:queue_id/insert",
		"POST /api/v1/queues/:queue_id/next",
		"POST /api/v1/owners/subscription/checkout",
	} {
		if !registered[want] {
			t.Errorf("route %q tidak terdaftar; perbarui internal/metrics/backend.go", want)
		}
	}
}
