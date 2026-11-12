package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Rasio sampling: nilai tidak valid atau di luar 0..1 jatuh ke 10%.
func TestSampleRatio(t *testing.T) {
	// tabel kasus: teks masukan -> rasio yang diharapkan
	for raw, want := range map[string]float64{"": 0.1, "abc": 0.1, "-1": 0.1, "1.5": 0.1, "0": 0, "0.25": 0.25, " 1 ": 1} {
		if got := sampleRatio(raw); got != want {
			t.Errorf("sampleRatio(%q) = %v, diharapkan %v", raw, got, want)
		}
	}
}

// Tanpa OTEL_EXPORTER_OTLP_ENDPOINT tracing mati dan shutdown aman dipanggil.
func TestSetup_DisabledWithoutEndpoint(t *testing.T) {
	// endpoint kosong = kondisi tanpa overlay observability
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown := Setup(context.Background(), "test")
	// tracing harus mati dan tidak ada yang dipasang
	if Enabled() {
		t.Fatal("tracing seharusnya mati tanpa endpoint")
	}
	// shutdown no-op harus aman dipanggil dan tidak mengembalikan galat
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown no-op mengembalikan galat: %v", err)
	}
}

// Endpoint tanpa skema ditambah http://.
func TestNormalizeEndpoint(t *testing.T) {
	// tanpa skema: ditambah http://
	if got := normalizeEndpoint("collector:4317"); got != "http://collector:4317" {
		t.Errorf("got %q", got)
	}
	// sudah berskema: tidak diubah
	if got := normalizeEndpoint("https://x:4317"); got != "https://x:4317" {
		t.Errorf("got %q", got)
	}
}

// newPipeline membuat provider yang mengekspor lewat penyaring privasi ke exporter memori.
func newPipeline(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	t.Helper()
	// exporter memori menampung span yang lolos penyaring, supaya bisa diperiksa tes
	mem := tracetest.NewInMemoryExporter()
	// WithSyncer: span diekspor langsung saat selesai (tanpa batch), sehingga tes deterministik
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSyncer(newRedactingExporter(mem)))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return tp, mem
}

// attrMap mengubah daftar atribut menjadi peta kunci -> nilai teks.
func attrMap(kvs []attribute.KeyValue) map[string]string {
	m := map[string]string{}
	// String() mengubah nilai bertipe apa pun menjadi teks agar mudah dibandingkan
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.String()
	}
	return m
}

// Penyaring membuang atribut berisi path mentah, query, header, dan alamat klien, serta menyamarkan rahasia di
// nilai yang tersisa (atribut, event, dan deskripsi status).
func TestRedactingExporter(t *testing.T) {
	tp, mem := newPipeline(t)
	_, span := tp.Tracer("t").Start(context.Background(), "GET /display/:token")
	// atribut campuran: kunci terlarang (path, query, header, IP), kunci aman (route, status, request_id), dan
	// kunci aman yang nilainya berisi rahasia (note, jwt, qr)
	span.SetAttributes(
		attribute.String("url.path", "/display/SECRET-DISPLAY-TOKEN"),
		attribute.String("url.query", "token=QUERYSECRET"),
		attribute.String("http.target", "/x?token=abc"),
		attribute.String("http.request.header.authorization", "Bearer eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl"),
		attribute.String("client.address", "10.1.2.3"),
		attribute.String("http.route", "/display/:token"),
		attribute.Int("http.response.status_code", 200),
		attribute.String("note", "gagal token=RAHASIA123 dan Bearer abc.def.ghi"),
		attribute.String("jwt", "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl"),
		attribute.String("qr", "v1.eyJxaWQiOjEsInR5cCI6InN0YXRpYyJ9.FmWtUpCFsvTRFMU0KH8Op"),
		attribute.String("request_id", "abc123"),
	)
	// event exception juga bisa memuat rahasia dari teks galat
	span.AddEvent("exception", oteltrace.WithAttributes(attribute.String("exception.message", "gagal password=hunter2")))
	span.End()

	// ambil span yang sudah melewati penyaring
	spans := mem.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("diharapkan 1 span, dapat %d", len(spans))
	}
	got := attrMap(spans[0].Attributes)
	// kunci terlarang harus hilang
	for _, k := range []string{"url.path", "url.query", "http.target", "http.request.header.authorization", "client.address"} {
		if _, ok := got[k]; ok {
			t.Errorf("atribut %s seharusnya dibuang", k)
		}
	}
	// atribut yang aman harus tetap ada dengan nilai aslinya
	if got["http.route"] != "/display/:token" || got["http.response.status_code"] != "200" || got["request_id"] != "abc123" {
		t.Errorf("atribut yang aman harus tetap: %v", got)
	}
	// tidak ada nilai yang tersisa boleh memuat potongan rahasia
	for k, v := range got {
		for _, secret := range []string{"RAHASIA123", "eyJhbGci", "FmWtUpCF", "abc.def.ghi", "SECRET", "hunter2"} {
			if strings.Contains(v, secret) {
				t.Errorf("atribut %s masih memuat %q: %s", k, secret, v)
			}
		}
	}
	// event exception juga harus bersih
	for _, ev := range spans[0].Events {
		if strings.Contains(attrMap(ev.Attributes)["exception.message"], "hunter2") {
			t.Error("event masih memuat rahasia")
		}
	}
}

// Middleware: nama span memakai templat route, rahasia di path/query tidak keluar, dan /health serta SSE tidak dilacak.
func TestMiddleware_SpanNameRedactionAndFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tp, mem := newPipeline(t)
	// middleware membaca provider global saat dibuat, jadi dipasang dulu
	otel.SetTracerProvider(tp)
	engine := gin.New()
	engine.Use(Middleware("test-svc"))
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	engine.GET("/display/:token", ok)
	engine.GET("/health", ok)
	engine.GET("/queues/:id/event", ok)

	// tiga request: biasa (dilacak), probe health (tidak), dan SSE (tidak)
	for _, path := range []string{"/display/SECRET-DISPLAY-TOKEN?token=QUERYSECRET", "/health", "/queues/1/event"} {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	// hanya request biasa yang boleh menghasilkan span
	spans := mem.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("hanya request biasa yang dilacak: diharapkan 1 span, dapat %d", len(spans))
	}
	// nama span berupa templat route, tanpa token
	if !strings.Contains(spans[0].Name, "/display/:token") {
		t.Errorf("nama span harus memuat templat route: %q", spans[0].Name)
	}
	if strings.Contains(spans[0].Name, "SECRET") {
		t.Errorf("nama span memuat rahasia: %q", spans[0].Name)
	}
	// atribut hasil ekspor tidak boleh memuat token dari path maupun query
	for k, v := range attrMap(spans[0].Attributes) {
		if strings.Contains(v, "SECRET") || strings.Contains(v, "QUERYSECRET") {
			t.Errorf("atribut %s memuat rahasia: %s", k, v)
		}
	}
}
