package telemetry

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Middleware membuat span server untuk tiap request HTTP (otelgin) dan membaca traceparent dari pemanggil. Nama span
// memakai TEMPLAT route (mis. "GET /api/v1/display/:token"), bukan path mentah. Probe /health, /ready, dan koneksi
// SSE (berakhiran /event) tidak dilacak: SSE berumur berjam-jam dan tidak boleh membuat span per koneksi.
// Atribut berisi path mentah, query, dan alamat klien dibuang saat ekspor (lihat redact.go).
func Middleware(
	service string,
) gin.HandlerFunc {
	return otelgin.Middleware(service,
		// filter menentukan request mana yang dilacak: true = dilacak
		otelgin.WithFilter(func(r *http.Request) bool {
			// path request (tanpa query string)
			p := r.URL.Path
			// probe liveness dipanggil terus oleh Docker dan SSE berumur panjang: keduanya tidak dilacak
			return p != "/health" && p != "/ready" && !strings.HasSuffix(p, "/event")
		}),
	)
}
