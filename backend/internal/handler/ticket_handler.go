package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/model"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// TicketService adalah logika tiket dan antrian (dipenuhi oleh service).
type TicketService interface {
	// JoinQueue mendaftarkan user ke antrian dan mengembalikan tiket barunya.
	JoinQueue(
		ctx context.Context,
		queueID,
		userID int64,
	) (model.Ticket, error)
	// NextTicket memanggil tiket berikutnya di counter. expected (bila tidak nil) memastikan counter belum berubah.
	NextTicket(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		expected *int64,
	) (model.Ticket, error)
	// RecallTicket memanggil ulang tiket yang sedang dilayani di counter.
	RecallTicket(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		expected *int64,
	) (model.Ticket, error)
	// SkipTicket melewati tiket yang sedang dilayani di counter.
	SkipTicket(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		expected *int64,
	) (model.Ticket, error)
	// CancelTicket membatalkan tiket aktif milik user di antrian.
	CancelTicket(
		ctx context.Context,
		queueID,
		userID int64,
	) (model.Ticket, error)
	// RemoveTicket menghapus tiket tertentu dari antrian (oleh owner).
	RemoveTicket(
		ctx context.Context,
		queueID,
		requesterID,
		ticketID int64,
	) (model.Ticket, error)
	// InsertTicket menyisipkan akun terdaftar (lewat email atau username) ke posisi tertentu.
	InsertTicket(
		ctx context.Context,
		queueID,
		requesterID int64,
		identifier string,
		position int,
	) (model.Ticket, error)
	// ListTickets mengambil daftar tiket antrian, bisa difilter berdasarkan status.
	ListTickets(
		ctx context.Context,
		queueID,
		requesterID int64,
		statuses []string,
	) ([]model.Ticket, error)
	// QueueStatus mengambil ringkasan antrian dan posisi tiket requester.
	QueueStatus(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.QueueStatus, error)
	// ListMyTickets mengambil tiket milik user di semua antrian.
	ListMyTickets(
		ctx context.Context,
		userID int64,
	) ([]model.UserTicket, error)
}

// JoinQRService adalah layanan yang memverifikasi token join dari QR code.
type JoinQRService interface {
	// ResolveJoinToken mencari antrian dari kode join lalu mengembalikannya.
	ResolveJoinToken(
		ctx context.Context,
		token string,
	) (model.Queue, error)
	// AfterJoin dipanggil setelah join berhasil; kode dinamis yang dipakai diganti dengan yang baru.
	AfterJoin(
		ctx context.Context,
		queueID int64,
		token string,
	) error
}

// TicketHandler menangani rute tiket: join, preview, panggil, lewati, batal, sisip, hapus, dan daftar.
type TicketHandler struct {
	// service berisi logika tiket dan antrian.
	service TicketService
	// qr memverifikasi token join. Nil bila dibuat lewat NewTicketHandler (tanpa fitur QR).
	qr JoinQRService
}

// NewTicketHandler membuat TicketHandler tanpa fitur QR.
func NewTicketHandler(
	service TicketService,
) *TicketHandler {
	// isi handler dengan service saja; qr tetap nil
	return &TicketHandler{
		service: service,
	}
}

// NewTicketHandlerWithQR membangun handler tiket dengan layanan QR yang
// menyediakan verifikasi token join.
func NewTicketHandlerWithQR(
	service TicketService,
	qr JoinQRService,
) *TicketHandler {
	// isi handler dengan service dan layanan QR
	return &TicketHandler{
		service: service,
		qr:      qr,
	}
}

// Join: POST /api/v1/join. Customer yang sudah login mendaftar ke antrian memakai token join.
func (h *TicketHandler) Join(
	c *gin.Context,
) {

	// ID user yang login
	ctx := c.Request.Context()
	userID := c.GetInt64("user_id")

	// ambil token dari query atau body; kosong berarti request tidak lengkap
	token := tokenFrom(c)
	if token == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// verifikasi token dan cari antrian yang dituju; error diterjemahkan ke respons HTTP
	queue, err := h.qr.ResolveJoinToken(
		ctx,
		token,
	)
	if err != nil {
		h.respondJoinTokenError(c, err)
		return
	}

	// daftarkan user ke antrian tersebut; error diterjemahkan ke respons HTTP
	ticket, err := h.service.JoinQueue(
		ctx,
		queue.ID,
		userID,
	)
	if err != nil {
		h.respondJoinError(c, err)
		return
	}

	// tiket sudah dibuat: ganti kode dinamis yang dipakai. Galat di sini tidak membatalkan join,
	// kode lama tetap berlaku sampai masa berlakunya habis.
	_ = h.qr.AfterJoin(ctx, queue.ID, token)

	// berhasil: kirim tiket baru
	respondData(
		c,
		http.StatusOK,
		dto.NewTicketResponse(ticket),
	)
}

// tokenFrom: token join dari query `t` (mis. GET /api/v1/join/preview?t=...) atau JSON body (POST /api/v1/join {t}).
func tokenFrom(c *gin.Context) string {
	// utamakan query ?t=, karena dipakai oleh preview (GET)
	if t := c.Query("t"); t != "" {
		return t
	}
	// bila tidak ada di query, coba baca dari body JSON
	var req dto.JoinByTokenRequest
	if err := c.ShouldBindJSON(&req); err == nil {
		return req.Token
	}
	// tidak ditemukan di mana pun
	return ""
}

// Preview: GET /api/v1/join/preview?t=... — info publik antrian dari token join.
func (h *TicketHandler) Preview(
	c *gin.Context,
) {

	// siapkan context request
	ctx := c.Request.Context()

	// ambil token dari query atau body; kosong berarti request tidak lengkap
	token := tokenFrom(c)
	if token == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	// verifikasi token dan cari antrian yang dituju; error diterjemahkan ke respons HTTP
	queue, err := h.qr.ResolveJoinToken(
		ctx,
		token,
	)
	if err != nil {
		h.respondJoinTokenError(c, err)
		return
	}

	// kirim info publik antrian saja, tanpa data owner
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action": "join_preview",
			"queue":  dto.NewPublicQueueResponse(queue),
		},
	)
}

// respondJoinTokenError memetakan error verifikasi token join ke response API.
func (h *TicketHandler) respondJoinTokenError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error; setiap case langsung mengirim respons dan keluar
	switch {
	// antrian tidak ditemukan: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		return
	// token tidak valid atau sudah dicabut: 404
	case errors.Is(err, apperror.ErrQRInvalid):
		respondError(c, http.StatusNotFound, "QR_INVALID", "invalid or revoked qr token")
		return
	// token sudah kedaluwarsa: 410
	case errors.Is(err, apperror.ErrQRExpired):
		respondError(c, http.StatusGone, "QR_EXPIRED", "qr token expired")
		return
	// qr-service tidak bisa dihubungi: 503
	case errors.Is(err, apperror.ErrQRUnavailable):
		respondError(c, http.StatusServiceUnavailable, "QR_UNAVAILABLE", "qr service is unavailable")
		return
	}
	// error lain yang tidak masuk daftar di atas: 500 tanpa detail (nil tidak melakukan apa-apa)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
}

// respondJoinError memetakan error join (token sudah divalidasi di atas).
func (h *TicketHandler) respondJoinError(
	c *gin.Context,
	err error,
) {
	// pilih respons sesuai jenis error; setiap case langsung mengirim respons dan keluar
	switch {
	// antrian tidak ditemukan: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		return
	// email belum diverifikasi: 403
	case errors.Is(err, apperror.ErrEmailNotVerified):
		respondError(c, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "verify your email before joining a queue")
		return
	// user sudah punya tiket aktif di antrian ini: 409
	case errors.Is(err, apperror.ErrAlreadyJoined):
		respondError(c, http.StatusConflict, "ALREADY_JOINED", "you have already joined this queue")
		return
	// antrian sudah ditutup: 409
	case errors.Is(err, apperror.ErrQueueClosed):
		respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		return
	// percobaan ulang untuk mendapat nomor habis: 503
	case errors.Is(err, apperror.ErrRetryExhausted):
		respondError(c, http.StatusServiceUnavailable, "RETRY_EXHAUSTED", "failed to join queue after exhausting all retries")
		return
	}
	// error lain yang tidak masuk daftar di atas: 500 tanpa detail (nil tidak melakukan apa-apa)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
}

// Next: POST /api/v1/queues/:queue_id/next — owner memanggil nomor berikutnya.
// Body opsional `{ "counter_id": 0 }`; 0 = counter default milik queue.
func (h *TicketHandler) Next(
	c *gin.Context,
) {

	// ambil queue_id dari URL dan ubah menjadi angka; gagal berarti bukan angka
	ctx := c.Request.Context()
	queueIDStr := c.Param("queue_id")
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

	// ambil counter_id dan expected_ticket_id dari body (opsional)
	counterID, expected := counterActionBody(c)

	// panggil nomor berikutnya untuk owner yang login
	ticket, err := h.service.NextTicket(
		ctx,
		queueID,
		c.GetInt64("user_id"),
		counterID,
		expected,
	)
	// counter sudah dimajukan dari perangkat lain: 409
	if errors.Is(err, apperror.ErrCounterStateChanged) {
		respondError(c, http.StatusConflict, "COUNTER_STATE_CHANGED", "this counter was just advanced from another device")
		return
	}
	// ada error lain
	if err != nil {
		// antrian tidak ditemukan: 404
		if errors.Is(err, apperror.ErrNotFound) {
			respondError(
				c,
				http.StatusNotFound,
				"NOT_FOUND",
				"queue not found",
			)
			return
		}
		// tidak ada tiket yang menunggu: 409
		if errors.Is(err, apperror.ErrQueueEmpty) {
			respondError(
				c,
				http.StatusConflict,
				"QUEUE_EMPTY",
				"queue is currently empty",
			)
			return
		}
		// error lain: 500 tanpa detail
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// berhasil: kirim tiket yang sekarang dilayani
	respondData(
		c,
		http.StatusOK,
		dto.NewTicketResponse(ticket),
	)
}

// Skip: POST /api/v1/queues/:queue_id/skip — owner melewati nomor yang sedang dilayani
// dan langsung memanggil nomor berikutnya.
func (h *TicketHandler) Skip(c *gin.Context) {
	// ambil queue_id dari URL; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// ambil counter_id dan expected_ticket_id dari body (opsional)
	counterID, expected := counterActionBody(c)
	// lewati tiket yang sedang dilayani di counter tersebut
	ticket, err := h.service.SkipTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID, expected)
	// counter sudah dimajukan dari perangkat lain: 409
	if errors.Is(err, apperror.ErrCounterStateChanged) {
		respondError(c, http.StatusConflict, "COUNTER_STATE_CHANGED", "this counter was just advanced from another device")
		return
	}
	// ada error
	if err != nil {
		switch {
		// antrian tidak ditemukan: 404
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		// tidak ada tiket yang sedang dilayani untuk dilewati: 409
		case errors.Is(err, apperror.ErrNothingToSkip):
			respondError(c, http.StatusConflict, "NOTHING_TO_SKIP", "no ticket is being served")
		// tidak ada tiket menunggu untuk dipanggil berikutnya: 409
		case errors.Is(err, apperror.ErrQueueEmpty):
			respondError(c, http.StatusConflict, "QUEUE_EMPTY", "queue is currently empty")
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	// berhasil: kirim tiket terakhir yang dilewati
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

// Cancel: POST /api/v1/queues/:queue_id/cancel — customer membatalkan nomornya sendiri (4.9).
// Tanpa tiket aktif, atau sudah tidak aktif: 200 dengan keadaan tiket terakhir (no-op).
func (h *TicketHandler) Cancel(c *gin.Context) {
	// ambil queue_id dari URL; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// batalkan tiket aktif milik user yang login
	ticket, err := h.service.CancelTicket(c.Request.Context(), queueID, c.GetInt64("user_id"))
	// ada error
	if err != nil {
		switch {
		// antrian tidak ditemukan: 404
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or ticket not found")
		// antrian sudah ditutup: 409
		case errors.Is(err, apperror.ErrQueueClosed):
			respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	// berhasil: kirim keadaan tiket terakhir
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

// insertTicketRequest adalah body JSON untuk menyisipkan akun ke antrian.
type insertTicketRequest struct {
	// Identifier adalah email atau username akun yang akan disisipkan.
	Identifier string `json:"identifier"`
	// Position adalah posisi tujuan di antrian, mulai dari 1.
	Position int `json:"position"`
}

// Insert: POST /api/v1/queues/:queue_id/insert — owner menyisipkan akun terdaftar di posisi tertentu (US-10).
func (h *TicketHandler) Insert(c *gin.Context) {
	// ambil queue_id dari URL; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// baca body JSON; gagal berarti format request salah
	var req insertTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// buang spasi di awal dan akhir identifier
	identifier := strings.TrimSpace(req.Identifier)
	// identifier kosong atau posisi di bawah 1: tolak
	if identifier == "" || req.Position < 1 {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "identifier is required and position must be at least 1")
		return
	}
	// sisipkan akun ke posisi yang diminta; error diterjemahkan ke respons HTTP
	ticket, err := h.service.InsertTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), identifier, req.Position)
	if err != nil {
		switch {
		// antrian atau akun tidak ditemukan: 404
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or account not found")
		// username dipakai lebih dari satu akun, jadi harus pakai email: 409
		case errors.Is(err, apperror.ErrAmbiguousAccount):
			respondError(c, http.StatusConflict, "AMBIGUOUS_ACCOUNT", "more than one account has this username; use the email instead")
		// akun sudah punya tiket aktif di antrian: 409
		case errors.Is(err, apperror.ErrAlreadyJoined):
			respondError(c, http.StatusConflict, "ALREADY_JOINED", "this account already has an active ticket in the queue")
		// antrian sudah ditutup: 409
		case errors.Is(err, apperror.ErrQueueClosed):
			respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		// posisi tidak valid: 400
		case errors.Is(err, apperror.ErrInvalidPosition):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", "position must be at least 1")
		// percobaan ulang habis: 503
		case errors.Is(err, apperror.ErrRetryExhausted):
			respondError(c, http.StatusServiceUnavailable, "RETRY_EXHAUSTED", "failed to insert after exhausting all retries")
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	// berhasil disisipkan: 201 Created
	respondData(c, http.StatusCreated, dto.NewTicketResponse(ticket))
}

// Remove: POST /api/v1/queues/:queue_id/tickets/:ticket_id/remove — owner menghapus nomor customer (4.10).
// Tiket sudah tidak aktif: 200 dengan keadaan tiket sekarang (no-op).
func (h *TicketHandler) Remove(c *gin.Context) {
	// ambil queue_id dari URL; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// ambil ticket_id dari URL; gagal berarti bukan angka
	ticketID, err := strconv.ParseInt(c.Param("ticket_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid ticket id")
		return
	}
	// hapus tiket dari antrian; error diterjemahkan ke respons HTTP
	ticket, err := h.service.RemoveTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), ticketID)
	if err != nil {
		switch {
		// antrian atau tiket tidak ditemukan: 404
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or ticket not found")
		// antrian sudah ditutup: 409
		case errors.Is(err, apperror.ErrQueueClosed):
			respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	// berhasil dihapus: kirim keadaan tiket
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

// Recall: POST /api/v1/queues/:queue_id/recall — owner memanggil ulang nomor yang sedang dilayani (push saja).
func (h *TicketHandler) Recall(c *gin.Context) {
	// ambil queue_id dari URL; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	// ambil counter_id dan expected_ticket_id dari body (opsional)
	counterID, expected := counterActionBody(c)
	// panggil ulang tiket yang sedang dilayani
	ticket, err := h.service.RecallTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID, expected)
	// counter sudah dimajukan dari perangkat lain: 409
	if errors.Is(err, apperror.ErrCounterStateChanged) {
		respondError(c, http.StatusConflict, "COUNTER_STATE_CHANGED", "this counter was just advanced from another device")
		return
	}
	// ada error
	if err != nil {
		switch {
		// antrian tidak ditemukan: 404
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		// tidak ada tiket yang sedang dilayani: 409
		case errors.Is(err, apperror.ErrNothingToRecall):
			respondError(c, http.StatusConflict, "NOTHING_TO_RECALL", "no ticket is being served")
		// baru saja dipanggil, tunggu 3 detik dulu: 409
		case errors.Is(err, apperror.ErrRecallTooSoon):
			respondError(c, http.StatusConflict, "RECALL_TOO_SOON", "recall is available 3 seconds after calling")
		// nomor ini sudah pernah dipanggil ulang: 409
		case errors.Is(err, apperror.ErrAlreadyRecalled):
			respondError(c, http.StatusConflict, "ALREADY_RECALLED", "this number was already recalled")
		// error lain: 500 tanpa detail
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	// berhasil: kirim tiket yang dipanggil ulang
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

// validTicketStatuses adalah daftar status tiket yang boleh dipakai sebagai filter. Nilai lain ditolak.
var validTicketStatuses = map[string]bool{
	"waiting":          true,
	"currently_served": true,
	"completed":        true,
	"cancelled":        true,
	"skipped":          true,
	"deleted":          true,
}

// parseStatusFilter membaca ?status=a&status=b atau ?status=a,b; false bila ada nilai tak dikenal.
func parseStatusFilter(c *gin.Context) ([]string, bool) {
	// kumpulkan status yang valid
	var statuses []string
	// setiap nilai ?status= bisa berisi beberapa status yang dipisah koma
	for _, raw := range c.QueryArray("status") {
		for _, st := range strings.Split(raw, ",") {
			// buang spasi di sekitar status
			st = strings.TrimSpace(st)
			// bagian kosong (misalnya koma berlebih) dilewati
			if st == "" {
				continue
			}
			// status tidak dikenal: seluruh filter ditolak
			if !validTicketStatuses[st] {
				return nil, false
			}
			statuses = append(statuses, st)
		}
	}
	// semua status valid (atau tidak ada filter sama sekali)
	return statuses, true
}

// List: GET /api/v1/queues/:queue_id/tickets?status=... — daftar tiket antrian untuk owner, bisa difilter status.
func (h *TicketHandler) List(
	c *gin.Context,
) {

	// ambil queue_id dari URL dan ubah menjadi angka; gagal berarti bukan angka
	ctx := c.Request.Context()
	queueIDStr := c.Param("queue_id")
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

	// baca filter status dari query; ada nilai tidak dikenal berarti request salah
	statuses, ok := parseStatusFilter(c)
	if !ok {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid ticket status",
		)
		return
	}

	// ID user yang login
	requesterID := c.GetInt64("user_id")

	// ambil daftar tiket sesuai filter
	tickets, err := h.service.ListTickets(
		ctx,
		queueID,
		requesterID,
		statuses,
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

	// kirim daftar tiket untuk owner
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action":   "list_tickets",
			"queue_id": queueID,
			"tickets":  dto.NewOwnerTicketResponses(tickets), // memuat catatan owner; endpoint ini owner saja
		},
	)
}

// Status: GET /api/v1/queues/:queue_id/status — ringkasan antrian dan posisi tiket milik requester.
func (h *TicketHandler) Status(
	c *gin.Context,
) {

	// ubah queue_id menjadi angka; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(
		c.Param("queue_id"),
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

	// ambil ringkasan antrian untuk user yang login
	status, err := h.service.QueueStatus(
		c.Request.Context(),
		queueID,
		c.GetInt64("user_id"),
	)
	// antrian tidak ditemukan atau tidak boleh dilihat: 404
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

	// ubah ke bentuk respons DTO, lalu susun field yang dikirim ke klien
	body := dto.NewQueueStatusResponse(status)
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action":               "queue_status",
			"queue":                body.Queue,
			"my_ticket":            body.MyTicket,
			"current_number":       body.CurrentNumber,
			"current_counter_name": body.CurrentCounterName,
			"serving":              body.Serving,
			"waiting_ahead":        body.WaitingAhead,
			"previous_number":      body.PreviousNumber,
			"next_number":          body.NextNumber,
		},
	)
}

// MyTickets: GET /api/v1/users/me/tickets — tiket saya di semua antrian.
func (h *TicketHandler) MyTickets(
	c *gin.Context,
) {
	// ambil tiket milik user yang login di semua antrian; error berarti error server
	tickets, err := h.service.ListMyTickets(
		c.Request.Context(),
		c.GetInt64("user_id"),
	)
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	// kirim daftar tiket dalam bentuk respons DTO
	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action":  "list_my_tickets",
			"tickets": dto.NewUserTicketResponses(tickets),
		},
	)
}

// counterActionBody: counter_id dan expected_ticket_id dari body Next, Skip, dan Recall. Body kosong atau rusak
// berarti counter default tanpa pengecekan (klien lama).
func counterActionBody(c *gin.Context) (int64, *int64) {
	// baca body JSON; gagal (termasuk body kosong) berarti pakai nilai bawaan
	var req dto.NextTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return 0, nil
	}
	// kembalikan counter_id dan expected_ticket_id dari body
	return req.CounterID, req.ExpectedTicketID
}
