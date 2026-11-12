package telemetry

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// defaultSampleRatio: bagian trace yang direkam bila OTEL_TRACES_SAMPLER_ARG kosong atau tidak valid (10%).
// Angka kecil menjaga RAM Tempo dan overhead aplikasi; naikkan ke 1 hanya saat menyelidiki masalah.
const defaultSampleRatio = 0.1

// enabled: true hanya setelah Setup berhasil memasang exporter. Dibaca Enabled() untuk memutuskan apakah instrumentasi
// opsional (mis. pembungkus database dan middleware tracing) perlu dipasang. Ditulis sekali saat start, sebelum
// ada request, jadi tidak perlu kunci.
var enabled bool

// Enabled melaporkan apakah tracing aktif (OTEL_EXPORTER_OTLP_ENDPOINT terisi dan exporter berhasil dibuat).
// Tanpa itu semua instrumentasi no-op dan aplikasi tidak butuh observability untuk start.
func Enabled() bool { return enabled }

// Setup menyalakan tracing bila OTEL_EXPORTER_OTLP_ENDPOINT terisi: exporter OTLP gRPC ke collector, sampling head
// dari OTEL_TRACES_SAMPLER_ARG (bawaan 10%), dan propagator W3C (traceparent). Kembalikan fungsi shutdown yang
// mengirim sisa span, aman dipanggil walau tracing mati (no-op).
//
// Tidak pernah menggagalkan start: bila exporter tidak bisa dibuat, hanya peringatan yang dicatat dan service jalan
// tanpa tracing. Pengiriman span asinkron lewat BatchSpanProcessor dengan antrean terbatas: bila collector mati atau
// lambat, span dibuang dan request tidak pernah menunggu.
func Setup(
	ctx context.Context,
	service string,
) func(context.Context) error {
	// propagator W3C selalu dipasang agar header traceparent diteruskan walau service ini tidak merekam trace;
	// dengan begitu keputusan sampling dari service hulu tetap dihormati
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// endpoint kosong: tracing mati, provider global tetap no-op (tidak ada span, tidak ada biaya)
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		// fungsi shutdown kosong supaya pemanggil tidak perlu memeriksa nil
		return func(context.Context) error { return nil }
	}
	// exporter OTLP gRPC; skema http:// berarti tanpa TLS (jaringan internal Docker). Pembuatannya tidak memblokir:
	// koneksi dibuka malas, jadi collector yang belum hidup tidak menahan start aplikasi.
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpointURL(normalizeEndpoint(endpoint)),
		// batas waktu satu kali kirim; lewat dari ini batch dibuang, request tidak terpengaruh
		otlptracegrpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		// gagal membuat exporter bukan alasan menghentikan aplikasi: catat dan lanjut tanpa tracing
		slog.Warn("tracing disabled: cannot create OTLP exporter", "err", err)
		return func(context.Context) error { return nil }
	}
	// nama service dari OTEL_SERVICE_NAME bila ada, selain itu nama yang diberikan main
	name := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if name == "" {
		name = service
	}
	// resource = identitas pengirim span; service.name dipakai Tempo dan Grafana untuk mengelompokkan trace
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(name)))
	if err != nil {
		// gagal menggabung: pakai resource bawaan (nama service jadi unknown_service, tetap berfungsi)
		res = resource.Default()
	}
	// rasio head sampling dari env
	ratio := sampleRatio(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	tp := sdktrace.NewTracerProvider(
		// ParentBased: bila request datang dengan keputusan sampling dari service lain, ikuti; selain itu rasio head.
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
		// span disamarkan sebelum diekspor (pagar privasi); antrean dibatasi supaya tidak memakan RAM, dan span
		// dibuang (bukan menahan request) saat antrean penuh
		sdktrace.WithBatcher(newRedactingExporter(exp),
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithExportTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
	)
	// jadikan provider global agar otelgin, otelhttp, dan otelsql memakainya
	otel.SetTracerProvider(tp)
	// tandai aktif, lalu catat konfigurasi efektif (tanpa rahasia)
	enabled = true
	slog.Info("tracing enabled", "endpoint", endpoint, "sample_ratio", ratio)
	// Shutdown mengirim sisa span di antrean sebelum proses berhenti
	return tp.Shutdown
}

// normalizeEndpoint menambahkan skema http:// bila endpoint ditulis tanpa skema (mis. "collector:4317").
func normalizeEndpoint(
	endpoint string,
) string {
	// sudah berskema: pakai apa adanya (https:// berarti TLS)
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	// tanpa skema: anggap jaringan internal tanpa TLS
	return "http://" + endpoint
}

// sampleRatio membaca rasio 0..1 dari teks. Kosong, bukan angka, atau di luar rentang memakai 10%.
func sampleRatio(
	raw string,
) float64 {
	// ubah teks (tanpa spasi pinggir) ke angka desimal
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	// bukan angka atau di luar 0..1: pakai nilai bawaan agar salah ketik tidak mematikan atau membanjiri trace
	if err != nil || v < 0 || v > 1 {
		return defaultSampleRatio
	}
	return v
}
