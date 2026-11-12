package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// logLine menulis satu log lewat logger yang dibungkus traceHandler dan mengembalikan JSON-nya sebagai peta.
func logLine(t *testing.T, ctx context.Context) map[string]any {
	t.Helper()
	// tulis ke buffer, bukan stdout, agar bisa dibaca tes
	buf := &bytes.Buffer{}
	// With(...) memastikan pembungkus tetap ada pada handler turunan (jalur yang dipakai NewLogger)
	logger := slog.New(traceHandler{Handler: slog.NewJSONHandler(buf, nil)}).With("service", "test")
	logger.InfoContext(ctx, "pesan")
	// urai satu baris JSON hasilnya
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("bukan JSON: %v: %s", err, buf.String())
	}
	return m
}

// spanCtx membuat context berisi span context palsu dengan ID tetap; sampled menentukan apakah trace direkam.
func spanCtx(sampled bool) context.Context {
	// flag sampled menandakan trace ini direkam dan akan ada di Tempo
	flags := trace.TraceFlags(0)
	if sampled {
		flags = trace.FlagsSampled
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: flags,
	})
	return trace.ContextWithSpanContext(context.Background(), sc)
}

// Log dengan context ber-span yang direkam membawa trace_id dan span_id (dalam hex), dan atribut lain tetap ada.
func TestTraceHandler_AddsIDsForSampledSpan(t *testing.T) {
	m := logLine(t, spanCtx(true))
	// ID harus sama dengan yang dimasukkan di spanCtx, dalam format hex 32 dan 16 karakter
	if m["trace_id"] != "0102030405060708090a0b0c0d0e0f10" || m["span_id"] != "0102030405060708" {
		t.Fatalf("trace_id/span_id tidak sesuai: %v", m)
	}
	// atribut dari With() tidak boleh hilang akibat pembungkus
	if m["service"] != "test" {
		t.Fatalf("atribut service hilang: %v", m)
	}
}

// Trace yang tidak direkam atau context tanpa span tidak menambah field (trace itu tidak ada di Tempo).
func TestTraceHandler_NoIDsWithoutSampledSpan(t *testing.T) {
	for name, ctx := range map[string]context.Context{"tidak direkam": spanCtx(false), "tanpa span": context.Background()} {
		if m := logLine(t, ctx); m["trace_id"] != nil || m["span_id"] != nil {
			t.Errorf("%s: tidak boleh ada trace_id: %v", name, m)
		}
	}
}
