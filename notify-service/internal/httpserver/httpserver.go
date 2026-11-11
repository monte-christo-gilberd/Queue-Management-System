package httpserver

import (
	"net/http"
	"sync/atomic"
	"time"
)

// Health: status LISTEN yang dilaporkan lewat /health. 200 hanya bila LISTEN queue_events sedang tersambung.
type Health struct {
	// listening: true selama LISTEN aktif.
	listening atomic.Bool
}

// SetListening memperbarui status LISTEN: true bila tersambung, false bila belum atau sudah putus.
func (h *Health) SetListening(
	up bool,
) {
	// Simpan status terbaru agar handler health membacanya.
	h.listening.Store(up)
}

// Handler membuat router HTTP dengan satu endpoint GET /health.
func (h *Health) Handler() http.Handler {
	// Buat router HTTP untuk health check.
	mux := http.NewServeMux()
	// Handler health: 503 bila LISTEN belum atau sudah tidak tersambung, 200 bila tersambung.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		// LISTEN tidak aktif: balas 503 agar orkestrasi tahu service tidak sehat.
		if !h.listening.Load() {
			http.Error(w, "not listening", http.StatusServiceUnavailable)
			return
		}
		// LISTEN aktif: balas 200 dengan status ok.
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	// Kembalikan router.
	return mux
}

// New membuat server health di addr dengan batas waktu membaca header 5 detik.
func New(
	addr string,
	h *Health,
) *http.Server {
	// Susun server dengan router health dan batas waktu header.
	return &http.Server{Addr: addr, Handler: h.Handler(), ReadHeaderTimeout: 5 * time.Second}
}
