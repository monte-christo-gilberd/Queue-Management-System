package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// discardExporter membuang semua span: mengukur biaya pembuatan span dan penyaringan tanpa biaya jaringan.
type discardExporter struct{}

// ExportSpans tidak melakukan apa-apa.
func (discardExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error { return nil }

// Shutdown tidak melakukan apa-apa.
func (discardExporter) Shutdown(context.Context) error { return nil }

// benchRequest menjalankan b.N request ke satu handler ringan. Dengan handler yang nyaris kosong, selisih antar
// varian adalah biaya murni middleware tracing per request (batas atas; request nyata jauh lebih lama karena DB).
func benchRequest(b *testing.B, withTracing bool, ratio float64) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if withTracing {
		// provider sungguhan dengan sampler berrasio dan batch processor + penyaring privasi, seperti produksi
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
			sdktrace.WithBatcher(newRedactingExporter(discardExporter{}), sdktrace.WithMaxQueueSize(2048)),
		)
		defer func() { _ = tp.Shutdown(context.Background()) }()
		otel.SetTracerProvider(tp)
		engine.Use(Middleware("bench"))
	}
	engine.GET("/api/v1/queues/:queue_id/tickets", func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/queues/1/tickets?x=1", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		engine.ServeHTTP(httptest.NewRecorder(), req)
	}
}

// BenchmarkRequest_NoTracing: dasar, tanpa middleware tracing (kondisi tanpa overlay).
func BenchmarkRequest_NoTracing(b *testing.B) { benchRequest(b, false, 0) }

// BenchmarkRequest_Sample0: middleware terpasang tetapi tidak ada trace yang direkam.
func BenchmarkRequest_Sample0(b *testing.B) { benchRequest(b, true, 0) }

// BenchmarkRequest_Sample10: sampling bawaan 10%.
func BenchmarkRequest_Sample10(b *testing.B) { benchRequest(b, true, 0.1) }

// BenchmarkRequest_Sample100: semua trace direkam (kasus terburuk).
func BenchmarkRequest_Sample100(b *testing.B) { benchRequest(b, true, 1) }
