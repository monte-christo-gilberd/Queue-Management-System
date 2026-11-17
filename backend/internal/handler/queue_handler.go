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

// ticketPrefixSetter: kemampuan opsional service (mengubah prefix nomor tiket).
// Tidak semua service memilikinya, jadi handler mengeceknya dengan type assertion sebelum memanggil.
type ticketPrefixSetter interface {
	// SetTicketPrefix mengubah awalan nomor tiket antrian (misalnya "A" untuk A-001).
	SetTicketPrefix(ctx context.Context, queueID, requesterID int64, prefix string) error
}

// QueueService adalah logika antrian (dipenuhi oleh service).
type QueueService interface {
	// CreateQueue membuat antrian baru milik ownerID.
	CreateQueue(
		ctx context.Context,
		ownerID int64,
		queueName,
		queueDescription string,
	) (model.Queue, error)
	// ViewQueue mengambil antrian. Bool menandakan apakah requester adalah owner.
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, bool, error)
	// ListQueue mengambil daftar antrian milik requester.
	ListQueue(
		ctx context.Context,
		requesterID int64,
	) ([]model.Queue, error)
	// UpdateQueue mengubah nama dan deskripsi antrian.
	UpdateQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
		queueName,
		queueDescription string,
	) error
	// CloseQueue menutup antrian.
	CloseQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) error
	// GetSkipSettings mengambil pengaturan skip antrian.
	GetSkipSettings(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.SkipSettings, error)
	// UpdateSkipSettings mengubah pengaturan skip antrian dan mengembalikan nilai yang tersimpan.
	UpdateSkipSettings(
		ctx context.Context,
		queueID,
		requesterID int64,
		settings model.SkipSettings,
	) (model.SkipSettings, error)
}

// QueueQRService adalah logika kode QR antrian (dipenuhi oleh service yang berbicara dengan qr-service).
type QueueQRService interface {
	// GetQR mengambil status QR operator untuk antrian.
	GetQR(
		ctx context.Context,
		queueID int64,
		requesterID int64,
	) (dto.QueueQRResponse, error)
	// SetQRType mengubah mode QR antrian (statis atau dinamis).
	SetQRType(
		ctx context.Context,
		queueID int64,
		requesterID int64,
		qrType string,
	) (dto.QueueQRResponse, error)
	// ReplaceQR mengganti kode QR statis dengan yang baru.
	ReplaceQR(
		ctx context.Context,
		queueID int64,
		requesterID int64,
	) (dto.QueueQRResponse, error)
}

// QueueHandler menangani rute antrian, pengaturan skip, dan QR operator.
type QueueHandler struct {
	// service berisi logika antrian.
	service QueueService
	// qr berisi logika QR. Boleh nil (tanpa fitur QR).
	qr QueueQRService
}

// NewQueueHandlerWithQR membuat QueueHandler lengkap dengan fitur QR.
func NewQueueHandlerWithQR(
	service QueueService,
	qr QueueQRService,
) *QueueHandler {
	// isi handler dengan service dan layanan QR
	return &QueueHandler{
		service: service,
		qr:      qr,
	}
}

// Create: POST /api/v1/queues. Membuat antrian baru, dengan prefix nomor tiket bila diisi.
func (h *QueueHandler) Create(
	c *gin.Context,
) {
	// ID user yang login sebagai pemilik antrian
	requesterID := c.GetInt64("user_id")

	// baca body JSON; gagal berarti format request salah
	var req dto.CreateQueueRequest
	if err := c.BindJSON(&req); err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid request body",
		)
		return
	}

	// rapikan prefix nomor tiket; bila formatnya salah, tolak
	prefix, okPrefix := dto.NormalizeTicketPrefix(req.TicketPrefix)
	if !okPrefix {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "ticket_prefix must be 1-3 letters or digits")
		return
	}

	// buat antrian lalu, bila ada prefix dan service mendukungnya, simpan prefix tersebut
	queue, err := h.service.CreateQueue(
		c.Request.Context(),
		requesterID,
		req.QueueName,
		req.QueueDescription,
	)
	// antrian berhasil dibuat dan prefix diisi: pasang prefix bila service mendukung fitur itu
	if err == nil && prefix != "" {
		// cek apakah service punya kemampuan mengubah prefix
		if setter, ok := h.service.(ticketPrefixSetter); ok {
			// simpan prefix; bila berhasil, perbarui data antrian di respons
			if err = setter.SetTicketPrefix(c.Request.Context(), queue.ID, requesterID, prefix); err == nil {
				queue.TicketPrefix = prefix
			}
		}
	}
	// owner belum memverifikasi email: 403
	if errors.Is(err, apperror.ErrEmailNotVerified) {
		respondError(
			c,
			http.StatusForbidden,
			"EMAIL_NOT_VERIFIED",
			"verify your email before creating a queue",
		)
		return
	}
	// langganan owner tidak aktif: 402
	if errors.Is(err, apperror.ErrSubscriptionInactive) {
		respondError(
			c,
			http.StatusPaymentRequired,
			"SUBSCRIPTION_INACTIVE",
			"inactive owner subscription",
		)
		return
	}
	// data yang dirujuk tidak ditemukan: 404
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// berhasil dibuat: 201 Created bersama data antrian
	respondData(
		c,
		http.StatusCreated,
		gin.H{
			"action": "create_queue",
			"queue":  dto.NewQueueResponse(queue),
		},
	)
}

// Get: GET /api/v1/queues/:queue_id. Owner melihat data lengkap, pemegang tiket hanya info publik.
func (h *QueueHandler) Get(
	c *gin.Context,
) {
	// ID user yang login dan queue_id dari URL
	requesterID := c.GetInt64("user_id")
	queueIDStr := c.Param("queue_id")

	// ubah queue_id menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(
		queueIDStr,
		10,
		64,
	)
	if err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid queue id",
		)
		return
	}

	// ambil antrian beserta informasi apakah requester adalah owner
	queue, isOwner, err := h.service.ViewQueue(
		c.Request.Context(),
		queueID,
		requesterID,
	)
	// antrian tidak ada atau tidak boleh dilihat: 404
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// Pemegang tiket hanya melihat info publik queue (tanpa owner_id & izin).
	// bukan owner: kirim info publik saja
	if !isOwner {
		respondData(
			c,
			http.StatusOK,
			gin.H{
				"action": "get_queue",
				"queue":  dto.NewPublicQueueResponse(queue),
			},
		)
		return
	}

	// owner: kirim data lengkap
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action": "get_queue",
			"queue":  dto.NewQueueResponse(queue),
		},
	)
}

// List: GET /api/v1/queues. Daftar antrian milik user yang login.
func (h *QueueHandler) List(
	c *gin.Context,
) {
	// ID user yang login
	requesterID := c.GetInt64("user_id")

	// ambil daftar antrian milik user
	queues, err := h.service.ListQueue(
		c.Request.Context(),
		requesterID,
	)
	// data tidak ditemukan: 404
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// kirim daftar antrian dalam bentuk respons DTO
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action": "list_queues",
			"queues": dto.NewQueueResponses(queues),
		},
	)
}

// Update: PUT /api/v1/queues/:queue_id. Mengubah nama, deskripsi, dan prefix nomor tiket.
func (h *QueueHandler) Update(
	c *gin.Context,
) {
	// ID user yang login dan queue_id dari URL
	requesterID := c.GetInt64("user_id")
	queueIDStr := c.Param("queue_id")

	// ubah queue_id menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(
		queueIDStr,
		10,
		64,
	)
	if err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid queue id",
		)
		return
	}

	// baca body JSON; gagal berarti format request salah
	var req dto.UpdateQueueRequest
	if err := c.BindJSON(&req); err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid request body",
		)
		return
	}

	// rapikan prefix nomor tiket; bila formatnya salah, tolak
	prefix, okPrefix := dto.NormalizeTicketPrefix(req.TicketPrefix)
	if !okPrefix {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "ticket_prefix must be 1-3 letters or digits")
		return
	}

	// ubah nama dan deskripsi antrian
	err = h.service.UpdateQueue(
		c.Request.Context(),
		queueID,
		requesterID,
		req.QueueName,
		req.QueueDescription,
	)
	// perubahan berhasil dan prefix diisi: simpan prefix bila service mendukungnya
	if err == nil && prefix != "" {
		// cek apakah service punya kemampuan mengubah prefix
		if setter, ok := h.service.(ticketPrefixSetter); ok {
			err = setter.SetTicketPrefix(c.Request.Context(), queueID, requesterID, prefix)
		}
	}
	// langganan owner tidak aktif: 402
	if errors.Is(err, apperror.ErrSubscriptionInactive) {
		respondError(
			c,
			http.StatusPaymentRequired,
			"SUBSCRIPTION_INACTIVE",
			"inactive owner subscription",
		)
		return
	}
	// antrian tidak ditemukan atau bukan milik user: 404
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// berhasil diubah
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action":   "update_queue",
			"queue_id": queueID,
			"status":   "success",
		},
	)
}

// Close: POST /api/v1/queues/:queue_id/close. Menutup antrian.
func (h *QueueHandler) Close(
	c *gin.Context,
) {
	// ID user yang login dan queue_id dari URL
	requesterID := c.GetInt64("user_id")
	queueIDStr := c.Param("queue_id")

	// ubah queue_id menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(
		queueIDStr,
		10,
		64,
	)
	if err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid queue id",
		)
		return
	}

	// tutup antrian
	err = h.service.CloseQueue(
		c.Request.Context(),
		queueID,
		requesterID,
	)
	// antrian tidak ditemukan atau bukan milik user: 404
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	// error lain: 500 tanpa detail
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// berhasil ditutup
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action":   "close",
			"queue_id": queueID,
			"status":   "success",
		},
	)
}

// GetQR: GET /api/v1/queues/:queue_id/qr — status QR operator untuk satu antrian.
func (h *QueueHandler) GetQR(
	c *gin.Context,
) {
	// ID user yang login dan queue_id dari URL; gagal berarti bukan angka
	requesterID := c.GetInt64("user_id")
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}

	// ambil status QR; error diterjemahkan ke respons HTTP
	qr, err := h.qr.GetQR(c.Request.Context(), queueID, requesterID)
	if err != nil {
		h.respondQRError(c, err)
		return
	}

	// kirim status QR
	respondData(c, http.StatusOK, qr)
}

// respondSkipSettings mengirim pengaturan skip dalam bentuk respons DTO.
func (h *QueueHandler) respondSkipSettings(c *gin.Context, s model.SkipSettings) {
	// kirim pengaturan skip
	respondData(c, http.StatusOK, dto.NewSkipSettingsResponse(s))
}

// respondSkipError menerjemahkan error dari pengaturan skip menjadi respons HTTP.
func (h *QueueHandler) respondSkipError(c *gin.Context, err error) {
	// pilih respons sesuai jenis error
	switch {
	// antrian tidak ditemukan: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
	// langganan owner tidak aktif: 402
	case errors.Is(err, apperror.ErrSubscriptionInactive):
		respondError(c, http.StatusPaymentRequired, "SUBSCRIPTION_INACTIVE", "subscription is inactive")
	// antrian sudah ditutup: 409
	case errors.Is(err, apperror.ErrQueueClosed):
		respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
	// nilai pengaturan skip tidak valid: 400
	case errors.Is(err, apperror.ErrInvalidSkipSettings):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid skip settings")
	// error lain: 500 tanpa detail
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}

// GetSkipSettings: GET /api/v1/queues/:queue_id/settings/skip
func (h *QueueHandler) GetSkipSettings(c *gin.Context) {
	// ubah queue_id menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// ambil pengaturan skip untuk user yang login; error diterjemahkan ke respons HTTP
	s, err := h.service.GetSkipSettings(c.Request.Context(), queueID, c.GetInt64("user_id"))
	if err != nil {
		h.respondSkipError(c, err)
		return
	}
	// kirim pengaturan skip
	h.respondSkipSettings(c, s)
}

// UpdateSkipSettings: PUT /api/v1/queues/:queue_id/settings/skip
func (h *QueueHandler) UpdateSkipSettings(c *gin.Context) {
	// ubah queue_id menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// baca body JSON; gagal berarti format request salah
	var req dto.SkipSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// simpan pengaturan skip baru; error diterjemahkan ke respons HTTP
	s, err := h.service.UpdateSkipSettings(c.Request.Context(), queueID, c.GetInt64("user_id"), model.SkipSettings{
		Fallback:   req.Fallback,
		N:          req.N,
		MaxReturns: req.MaxReturns,
	})
	if err != nil {
		h.respondSkipError(c, err)
		return
	}
	// kirim pengaturan yang tersimpan
	h.respondSkipSettings(c, s)
}

// SetQRType: PUT /api/v1/queues/:queue_id/qr/type — set mode statis/dinamik.
func (h *QueueHandler) SetQRType(
	c *gin.Context,
) {
	// ID user yang login dan queue_id dari URL; gagal berarti bukan angka
	requesterID := c.GetInt64("user_id")
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}

	// baca body JSON yang berisi jenis QR; gagal berarti format request salah
	var req dto.SetQRTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// ubah jenis QR; error diterjemahkan ke respons HTTP
	qr, err := h.qr.SetQRType(c.Request.Context(), queueID, requesterID, req.Type)
	if err != nil {
		h.respondQRError(c, err)
		return
	}

	// kirim status QR yang baru
	respondData(c, http.StatusOK, qr)
}

// ReplaceQR: POST /api/v1/queues/:queue_id/qr/replace — mengganti kode statis.
func (h *QueueHandler) ReplaceQR(
	c *gin.Context,
) {
	// ID user yang login dan queue_id dari URL; gagal berarti bukan angka
	requesterID := c.GetInt64("user_id")
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}

	// ganti kode QR; error diterjemahkan ke respons HTTP
	qr, err := h.qr.ReplaceQR(c.Request.Context(), queueID, requesterID)
	if err != nil {
		h.respondQRError(c, err)
		return
	}

	// kirim status QR yang baru
	respondData(c, http.StatusOK, qr)
}

// respondQRError memetakan error QR ke response API.
func (h *QueueHandler) respondQRError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error; setiap case langsung mengirim respons dan keluar
	switch {
	// token QR tidak valid atau sudah dicabut: 404
	case errors.Is(err, apperror.ErrQRInvalid):
		respondError(c, http.StatusNotFound, "QR_INVALID", "invalid or revoked qr token")
		return
	// token QR sudah kedaluwarsa: 410
	case errors.Is(err, apperror.ErrQRExpired):
		respondError(c, http.StatusGone, "QR_EXPIRED", "qr token expired")
		return
	// jenis QR yang diminta tidak dikenal: 400
	case errors.Is(err, apperror.ErrQRTypeInvalid):
		respondError(c, http.StatusBadRequest, "QR_TYPE_INVALID", "invalid qr type")
		return
	// penggantian kode hanya boleh untuk QR statis: 409
	case errors.Is(err, apperror.ErrQRReplaceNotAllowed):
		respondError(c, http.StatusConflict, "QR_REPLACE_NOT_ALLOWED", "qr replacement is only allowed for static qr")
		return
	// qr-service tidak bisa dihubungi: 503
	case errors.Is(err, apperror.ErrQRUnavailable):
		respondError(c, http.StatusServiceUnavailable, "QR_UNAVAILABLE", "qr service is unavailable")
		return
	// antrian tidak ditemukan: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		return
	}
	// error lain yang tidak masuk daftar di atas: 500 tanpa detail (nil tidak melakukan apa-apa)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
}
