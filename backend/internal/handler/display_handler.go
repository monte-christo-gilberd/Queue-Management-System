package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"qms/internal/service"
	"qms/internal/sse"

	"github.com/gin-gonic/gin"
)

// DisplayService adalah logika layar publik antrian (dipenuhi oleh service).
type DisplayService interface {
	// Config mengambil konfigurasi layar publik antrian, hanya untuk owner.
	Config(ctx context.Context, queueID, requesterID int64) (model.QueueDisplay, error)
	// Configure mengaktifkan layar atau mengubah jumlah nomor yang ditampilkan.
	Configure(ctx context.Context, queueID, requesterID int64, upcoming, skipped int) (model.QueueDisplay, error)
	// Rotate membuat token layar publik baru; token lama tidak berlaku lagi.
	Rotate(ctx context.Context, queueID, requesterID int64) (model.QueueDisplay, error)
	// Disable mematikan layar publik antrian.
	Disable(ctx context.Context, queueID, requesterID int64) error
	// Snapshot mengambil data layar publik memakai token.
	Snapshot(ctx context.Context, token string) (model.DisplaySnapshot, error)
	// QueueIDForToken mencari ID antrian yang terikat pada token layar publik.
	QueueIDForToken(ctx context.Context, token string) (int64, error)
}

// maxDisplaysPerQueue: sambungan SSE layar publik bersamaan per antrian. Token yang bocor tidak boleh dipakai
// untuk membuka sambungan tanpa batas.
const maxDisplaysPerQueue = 20

// DisplayHandler menangani konfigurasi layar publik (untuk owner) dan rute publik layar tanpa login.
type DisplayHandler struct {
	// service berisi logika layar publik.
	service DisplayService
	// hub menyebarkan sinyal ke layar publik lewat SSE.
	hub *sse.Hub
	// maxConn adalah batas sambungan SSE layar publik per antrian.
	maxConn int
}

// NewDisplayHandler membuat DisplayHandler dengan service dan hub, dan batas sambungan bawaan.
func NewDisplayHandler(service DisplayService, hub *sse.Hub) *DisplayHandler {
	// isi handler dengan batas sambungan bawaan
	return &DisplayHandler{service: service, hub: hub, maxConn: maxDisplaysPerQueue}
}

// respondDisplayError menerjemahkan error dari service layar publik menjadi respons HTTP.
func respondDisplayError(c *gin.Context, err error) {
	// pilih respons sesuai jenis error
	switch {
	// layar publik belum ada atau token tidak berlaku: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "display not found")
	// jumlah nomor di luar 0 sampai 10: 400
	case errors.Is(err, apperror.ErrInvalidDisplayConfig):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "display counts must be between 0 and 10")
	// error lain: 500 tanpa detail
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// ---- owner (butuh login) ----

// Get: GET /api/v1/queues/:queue_id/display. 404 bila belum diaktifkan.
func (h *DisplayHandler) Get(c *gin.Context) {
	// ambil ID antrian dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	// ambil konfigurasi layar untuk owner yang login
	d, err := h.service.Config(c.Request.Context(), queueID, c.GetInt64("user_id"))
	if err != nil {
		respondDisplayError(c, err)
		return
	}
	// kirim konfigurasi layar
	respondData(c, http.StatusOK, dto.NewDisplayConfigResponse(d))
}

// Update: PUT /api/v1/queues/:queue_id/display. Mengaktifkan layar atau mengubah jumlahnya.
func (h *DisplayHandler) Update(c *gin.Context) {
	// ambil ID antrian dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	// baca body JSON; gagal berarti format request salah
	var req dto.UpdateDisplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// ID user yang login
	userID := c.GetInt64("user_id")
	// Jumlah yang tidak dikirim: pakai yang sekarang bila layar ada, kalau tidak nilai bawaan.
	upcoming, skipped := service.DisplayDefaultUpcoming, 0
	// ambil konfigurasi yang sudah ada sebagai nilai awal
	if existing, err := h.service.Config(c.Request.Context(), queueID, userID); err == nil {
		upcoming, skipped = existing.UpcomingCount, existing.SkippedCount
		// layar belum ada (nilai bawaan dipakai), tetapi error lain dianggap gagal
	} else if !errors.Is(err, apperror.ErrNotFound) {
		respondDisplayError(c, err)
		return
	}
	// jumlah nomor berikutnya dikirim: pakai nilai dari request
	if req.UpcomingCount != nil {
		upcoming = *req.UpcomingCount
	}
	// jumlah nomor yang dilewati dikirim: pakai nilai dari request
	if req.SkippedCount != nil {
		skipped = *req.SkippedCount
	}
	// simpan konfigurasi layar; error diterjemahkan ke respons HTTP
	d, err := h.service.Configure(c.Request.Context(), queueID, userID, upcoming, skipped)
	if err != nil {
		respondDisplayError(c, err)
		return
	}
	// kirim konfigurasi yang tersimpan
	respondData(c, http.StatusOK, dto.NewDisplayConfigResponse(d))
}

// Rotate: POST /api/v1/queues/:queue_id/display/rotate. Token lama berhenti berlaku.
func (h *DisplayHandler) Rotate(c *gin.Context) {
	// ambil ID antrian dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	// buat token baru; error diterjemahkan ke respons HTTP
	d, err := h.service.Rotate(c.Request.Context(), queueID, c.GetInt64("user_id"))
	if err != nil {
		respondDisplayError(c, err)
		return
	}
	// kirim konfigurasi dengan token yang baru
	respondData(c, http.StatusOK, dto.NewDisplayConfigResponse(d))
}

// Disable: DELETE /api/v1/queues/:queue_id/display. Idempoten.
func (h *DisplayHandler) Disable(c *gin.Context) {
	// ambil ID antrian dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	// matikan layar; error diterjemahkan ke respons HTTP
	if err := h.service.Disable(c.Request.Context(), queueID, c.GetInt64("user_id")); err != nil {
		respondDisplayError(c, err)
		return
	}
	// berhasil dimatikan (atau memang sudah mati): 204 tanpa isi
	c.Status(http.StatusNoContent)
}

// ---- publik (tanpa login, token di path) ----

// Snapshot: GET /api/v1/display/:token. Token tidak berlaku: 404 yang sama untuk semua sebab.
func (h *DisplayHandler) Snapshot(c *gin.Context) {
	// ambil data layar publik memakai token di path; error diterjemahkan ke respons HTTP
	snap, err := h.service.Snapshot(c.Request.Context(), c.Param("token"))
	if err != nil {
		respondDisplayError(c, err)
		return
	}
	// data layar berubah-ubah, jadi jangan di-cache
	c.Header("Cache-Control", "no-store")
	// kirim snapshot dalam bentuk respons DTO
	respondData(c, http.StatusOK, dto.NewDisplaySnapshotResponse(snap))
}

// Events: GET /api/v1/display/:token/event. SSE tanpa login. Berada di room terpisah (sse.DisplayRoom) yang hanya
// menerima sinyal "display_changed" tanpa data; payload tiket di room antrian biasa tidak pernah sampai ke sini.
func (h *DisplayHandler) Events(c *gin.Context) {
	// cari antrian yang terikat pada token; token tidak berlaku berarti error
	queueID, err := h.service.QueueIDForToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		respondDisplayError(c, err)
		return
	}
	// kunci room khusus layar publik untuk antrian ini
	room := sse.DisplayRoom(queueID)
	// batas sambungan sudah penuh: tolak dengan 429
	if h.hub.RoomSize(room) >= h.maxConn {
		respondError(c, http.StatusTooManyRequests, "TOO_MANY_DISPLAYS", "too many displays are connected to this queue")
		return
	}

	// header khusus SSE: tipe stream, jangan di-cache, jaga koneksi, dan cegah proxy mem-buffer
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// buat klien SSE untuk room layar publik, lalu daftarkan ke hub
	client := sse.NewClient(h.hub, c.Writer, room)
	// pendaftaran gagal berarti hub sudah berhenti atau antrean penuh
	if !h.hub.Subscribe(client) {
		respondError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "service is not available")
		return
	}
	// lepas klien dari room saat koneksi putus
	defer h.hub.Unsubscribe(client)
	// kirim event ke layar sampai koneksi putus atau server berhenti (fungsi ini memblokir)
	client.Serve(c.Request)
}
