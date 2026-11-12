package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery menangkap panic di handler, mencatatnya lewat slog (JSON, lengkap dengan request_id dan stack), lalu
// membalas 500 dengan body error yang seragam. Pasang SETELAH Logger agar request yang panic tetap tercatat
// sebagai baris akses berstatus 500.
func Recovery(
	logger *slog.Logger,
) gin.HandlerFunc {
	// handler dijalankan untuk setiap request
	return func(c *gin.Context) {
		// defer berjalan saat handler selesai, termasuk saat terjadi panic
		defer func() {
			// recover mengembalikan nilai panic; nil berarti tidak ada panic
			r := recover()
			if r == nil {
				return
			}
			// http.ErrAbortHandler sengaja dipakai net/http untuk memutus koneksi: lempar ulang, jangan dicatat
			if r == http.ErrAbortHandler {
				panic(r)
			}
			// catat panic beserta stack agar penyebabnya bisa dilacak; request_id menghubungkannya ke baris akses,
			// dan ErrorContext membawa trace_id bila request sedang dilacak
			logger.ErrorContext(c.Request.Context(), "panic recovered",
				"request_id", c.GetString("request_id"),
				"panic", fmt.Sprint(r),
				"stack", string(debug.Stack()),
			)
			// balas 500 tanpa membocorkan detail panic ke klien; bentuk body sama dengan galat handler biasa
			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "internal server error",
					},
				},
			)
		}()
		// jalankan handler berikutnya; bila panic, defer di atas yang mengambil alih
		c.Next()
	}
}
