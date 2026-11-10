package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CounterService adalah logika CRUD counter (dipenuhi oleh service). Semua operasi hanya untuk owner antrian.
type CounterService interface {
	// List mengambil daftar counter beserta nomor yang sedang dilayani di tiap counter.
	List(
		ctx context.Context,
		queueID,
		requesterID int64,
	) ([]model.CounterWithServing, error)
	// Create membuat counter baru dengan nama name.
	Create(
		ctx context.Context,
		queueID,
		requesterID int64,
		name string,
	) (model.Counter, error)
	// Rename mengganti nama counter counterID.
	Rename(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		name string,
	) (model.Counter, error)
	// Delete menghapus counter counterID.
	Delete(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
	) error
}

// CounterHandler menangani rute CRUD counter antrian.
type CounterHandler struct {
	// service berisi logika counter.
	service CounterService
}

// NewCounterHandler membuat CounterHandler dengan service yang diberikan.
func NewCounterHandler(
	service CounterService,
) *CounterHandler {
	// isi handler dengan service
	return &CounterHandler{
		service: service,
	}
}

// counterNameRequest adalah body JSON untuk membuat dan mengganti nama counter.
type counterNameRequest struct {
	// Name adalah nama counter; kosong berarti nama otomatis (saat membuat).
	Name string `json:"name"`
}

// List: GET /api/v1/queues/:queue_id/counters — daftar counter beserta nomor yang sedang dilayani.
func (h *CounterHandler) List(c *gin.Context) {
	// ambil ID antrian dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	// ambil daftar counter untuk user yang login; error diterjemahkan ke respons HTTP
	counters, err := h.service.List(c.Request.Context(), queueID, c.GetInt64("user_id"))
	if err != nil {
		respondCounterError(c, err)
		return
	}
	// ubah setiap counter menjadi bentuk respons DTO, dengan kapasitas awal sesuai jumlah counter
	resp := make([]dto.CounterResponse, 0, len(counters))
	for _, counter := range counters {
		resp = append(resp, dto.NewCounterResponse(counter))
	}
	// kirim daftar counter bersama nama aksi dan ID antrian
	respondData(c, http.StatusOK, gin.H{"action": "list_counters", "queue_id": queueID, "counters": resp})
}

// Create: POST /api/v1/queues/:queue_id/counters — buat counter. Nama kosong = nama otomatis "Counter N".
func (h *CounterHandler) Create(c *gin.Context) {
	// ambil ID antrian dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ok := parseQueueID(c)
	if !ok {
		return
	}
	// baca body JSON yang berisi nama counter; gagal berarti format request salah
	var req counterNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// buat counter baru; error diterjemahkan ke respons HTTP
	counter, err := h.service.Create(c.Request.Context(), queueID, c.GetInt64("user_id"), req.Name)
	if err != nil {
		respondCounterError(c, err)
		return
	}
	// berhasil dibuat: 201 Created
	respondData(c, http.StatusCreated, dto.NewCounterFromModel(counter))
}

// Rename: PUT /api/v1/queues/:queue_id/counters/:counter_id — ganti nama (counter default boleh).
func (h *CounterHandler) Rename(c *gin.Context) {
	// ambil ID antrian dan ID counter dari URL; bila tidak valid, respons error sudah dikirim
	queueID, counterID, ok := parseQueueAndCounterID(c)
	if !ok {
		return
	}
	// baca body JSON yang berisi nama baru; gagal berarti format request salah
	var req counterNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// ganti nama counter; error diterjemahkan ke respons HTTP
	counter, err := h.service.Rename(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID, req.Name)
	if err != nil {
		respondCounterError(c, err)
		return
	}
	// berhasil diganti
	respondData(c, http.StatusOK, dto.NewCounterFromModel(counter))
}

// Delete: DELETE /api/v1/queues/:queue_id/counters/:counter_id — hapus counter. Default: 409.
func (h *CounterHandler) Delete(c *gin.Context) {
	// ambil ID antrian dan ID counter dari URL; bila tidak valid, respons error sudah dikirim
	queueID, counterID, ok := parseQueueAndCounterID(c)
	if !ok {
		return
	}
	// hapus counter; error (misalnya counter default) diterjemahkan ke respons HTTP
	if err := h.service.Delete(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID); err != nil {
		respondCounterError(c, err)
		return
	}
	// berhasil dihapus
	respondData(c, http.StatusOK, gin.H{"action": "delete_counter", "queue_id": queueID, "counter_id": counterID})
}

// parseQueueID membaca queue_id dari URL. Bila bukan angka, ia mengirim 400 dan mengembalikan ok false.
func parseQueueID(c *gin.Context) (int64, bool) {
	// ubah queue_id menjadi angka 64-bit; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return 0, false
	}
	// ID valid
	return queueID, true
}

// parseQueueAndCounterID membaca queue_id dan counter_id dari URL. Bila salah satu bukan angka, ia mengirim 400
// dan mengembalikan ok false.
func parseQueueAndCounterID(c *gin.Context) (int64, int64, bool) {
	// periksa queue_id lebih dulu
	queueID, ok := parseQueueID(c)
	if !ok {
		return 0, 0, false
	}
	// ubah counter_id menjadi angka 64-bit; gagal berarti bukan angka
	counterID, err := strconv.ParseInt(c.Param("counter_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid counter id")
		return 0, 0, false
	}
	// kedua ID valid
	return queueID, counterID, true
}

// respondCounterError menerjemahkan error dari service counter menjadi respons HTTP.
func respondCounterError(c *gin.Context, err error) {
	// pilih respons sesuai jenis error
	switch {
	// antrian atau counter tidak ditemukan: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or counter not found")
	// antrian sudah ditutup, counter tidak bisa diubah: 409
	case errors.Is(err, apperror.ErrQueueClosed):
		respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
	// nama counter sudah dipakai: 409
	case errors.Is(err, apperror.ErrDuplicateCounterName):
		respondError(c, http.StatusConflict, "DUPLICATE_COUNTER_NAME", "a counter with this name already exists")
	// jumlah counter sudah mencapai batas: 409
	case errors.Is(err, apperror.ErrCounterLimit):
		respondError(c, http.StatusConflict, "COUNTER_LIMIT", "a queue can have at most 10 counters")
	// counter default tidak boleh dihapus: 409
	case errors.Is(err, apperror.ErrDefaultCounter):
		respondError(c, http.StatusConflict, "DEFAULT_COUNTER", "the default counter cannot be deleted")
	// nama counter tidak memenuhi panjang 1 sampai 30 karakter: 400
	case errors.Is(err, apperror.ErrInvalidCounterName):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "counter name must be 1-30 characters")
	// error lain: 500 tanpa detail
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
