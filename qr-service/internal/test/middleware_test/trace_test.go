package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"qms/qr-service/internal/middleware"
)

// engineWithSpan membuat engine yang membuka span (seperti middleware tracing produksi) sebelum RequestID dan Logger,
// lalu mengembalikan perekam span dan buffer log JSON untuk diperiksa.
func engineWithSpan(t *testing.T) (*gin.Engine, *tracetest.SpanRecorder, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// perekam menyimpan span yang selesai di memori; AlwaysSample agar semua span terekam
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	// log ditulis ke buffer agar bisa dibaca tes
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	engine := gin.New()
	// middleware pertama meniru otelgin: buka span, taruh di context request, selesaikan setelah semua handler
	engine.Use(func(c *gin.Context) {
		ctx, span := tp.Tracer("t").Start(c.Request.Context(), "server")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		span.End()
	})
	// urutan produksi: RequestID lalu Logger, di dalam span
	engine.Use(middleware.RequestID(), middleware.Logger(logger))
	engine.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine, rec, buf
}

// RequestID mencatat request_id sebagai atribut span aktif, sehingga trace bisa dicari lewat request_id.
func TestRequestID_SetsSpanAttribute(t *testing.T) {
	engine, rec, _ := engineWithSpan(t)
	// request dengan ID dari nginx
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-Id", "req-trace-1")

	engine.ServeHTTP(httptest.NewRecorder(), req)

	// harus ada tepat satu span yang selesai
	ended := rec.Ended()
	if len(ended) != 1 {
		t.Fatalf("diharapkan 1 span, dapat %d", len(ended))
	}
	// cari atribut request_id dengan nilai yang dikirim
	found := false
	for _, kv := range ended[0].Attributes() {
		if string(kv.Key) == "request_id" && kv.Value.AsString() == "req-trace-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("atribut request_id tidak ada di span: %v", ended[0].Attributes())
	}
}

// Log akses memakai context request yang membawa span aktif (handler logging.NewLogger menambah trace_id dari sana;
// lihat internal/logging). Tes ini memastikan span dan log akses sama-sama terbentuk untuk request yang sama.
func TestLogger_UsesRequestContext(t *testing.T) {
	engine, rec, buf := engineWithSpan(t)

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	// span terbentuk
	if len(rec.Ended()) != 1 {
		t.Fatal("span tidak terbentuk")
	}
	// dan log akses berformat JSON dengan pesan "request"
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil || line["msg"] != "request" {
		t.Fatalf("log akses tidak sesuai: %s", buf.String())
	}
}
