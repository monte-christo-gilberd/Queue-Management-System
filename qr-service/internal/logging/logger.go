package logging

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger membuat logger JSON ke stdout dengan level dari LOG_LEVEL. Nilai tidak dikenal memakai info.
func NewLogger(
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
	// buat handler JSON ke stdout dengan level yang sudah dipilih
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
