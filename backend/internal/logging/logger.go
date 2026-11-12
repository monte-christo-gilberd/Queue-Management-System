package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// traceHandler menambahkan trace_id dan span_id ke setiap log yang ditulis dengan context yang membawa span
// (mis. logger.InfoContext(ctx, ...)), supaya Grafana bisa melompat dari baris log ke trace-nya. Hanya trace yang
// direkam (sampled) yang dicatat, karena trace yang tidak direkam tidak ada di Tempo.
type traceHandler struct {
	slog.Handler
}

// Handle menambah trace_id dan span_id bila ctx membawa span yang direkam, lalu meneruskan ke handler asli.
func (h traceHandler) Handle(
	ctx context.Context,
	r slog.Record,
) error {
	// span context dari ctx; tidak valid atau tidak direkam: tidak ada yang ditambahkan
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() && sc.IsSampled() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs mempertahankan pembungkus trace pada handler turunan (mis. hasil logger.With).
func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup mempertahankan pembungkus trace pada handler turunan.
func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{Handler: h.Handler.WithGroup(name)}
}

// NewLogger membuat logger JSON ke stdout dengan level dari LOG_LEVEL. Log yang ditulis dengan context ber-span membawa trace_id dan span_id. Setiap baris log membawa atribut
// service=<nama service> supaya log semua service bisa dipisahkan di Loki. Nilai level tidak dikenal memakai info.
func NewLogger(
	service string,
	level string,
) *slog.Logger {
	// level log yang dipakai handler, bawaannya info
	var lvl slog.Level
	// terjemahkan nama level dari env (tanpa memperhatikan huruf besar/kecil) ke slog.Level
	switch strings.ToLower(level) {
	// debug: semua log, termasuk yang tidak penting
	case "debug":
		// level terendah, termasuk log debug
		lvl = slog.LevelDebug
	// warn: hanya peringatan dan galat
	case "warn":
		lvl = slog.LevelWarn
	// error: hanya galat
	case "error":
		// level tertinggi, hanya galat
		lvl = slog.LevelError
	// nilai lain (termasuk info): info ke atas
	default:
		// info adalah bawaan
		lvl = slog.LevelInfo
	}
	// buat handler JSON ke stdout dengan level yang sudah dipilih, lalu tempelkan nama service ke semua baris
	// dibungkus traceHandler agar log yang memakai context ber-span otomatis membawa trace_id dan span_id
	return slog.New(traceHandler{Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})}).With("service", service)
}
