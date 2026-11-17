package telemetry

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Middleware membuat span server untuk tiap request HTTP (otelgin) dan membaca traceparent dari pemanggil. Nama span
// memakai TEMPLAT route (mis. "POST /internal/v1/send"), bukan path mentah. Probe /health tidak dilacak. Filter
// juga mengecualikan /ready dan path berakhiran /event (SSE), tetapi mail-service tidak punya rute itu; sisa salinan
// dari backend.
// Atribut berisi path mentah, query, dan alamat klien dibuang saat ekspor (lihat redact.go).
func Middleware(
	service string,
) gin.HandlerFunc {
	return otelgin.Middleware(service,
		// filter menentukan request mana yang dilacak: true = dilacak
		otelgin.WithFilter(func(r *http.Request) bool {
			// path request (tanpa query string)
			p := r.URL.Path
			// probe liveness dipanggil terus oleh Docker, jadi tidak dilacak (/ready dan /event tidak ada di mail-service)
			return p != "/health" && p != "/ready" && !strings.HasSuffix(p, "/event")
		}),
	)
}
