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

type TicketService interface {
	JoinQueue(
		ctx context.Context,
		queueID,
		userID int64,
	) (model.Ticket, error)
	NextTicket(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		expected *int64,
	) (model.Ticket, error)
	RecallTicket(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		expected *int64,
	) (model.Ticket, error)
	SkipTicket(
		ctx context.Context,
		queueID,
		requesterID,
		counterID int64,
		expected *int64,
	) (model.Ticket, error)
	CancelTicket(
		ctx context.Context,
		queueID,
		userID int64,
	) (model.Ticket, error)
	RemoveTicket(
		ctx context.Context,
		queueID,
		requesterID,
		ticketID int64,
	) (model.Ticket, error)
	InsertTicket(
		ctx context.Context,
		queueID,
		requesterID int64,
		identifier string,
		position int,
	) (model.Ticket, error)
	ListTickets(
		ctx context.Context,
		queueID,
		requesterID int64,
		statuses []string,
	) ([]model.Ticket, error)
	QueueStatus(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.QueueStatus, error)
	ListMyTickets(
		ctx context.Context,
		userID int64,
	) ([]model.UserTicket, error)
}

type JoinQRService interface {
	// ResolveJoinToken memverifikasi token join lalu mengembalikan antrian-nya.
	ResolveJoinToken(
		ctx context.Context,
		token string,
	) (model.Queue, error)
}

type TicketHandler struct {
	service TicketService
	qr      JoinQRService
}

func NewTicketHandler(
	service TicketService,
) *TicketHandler {
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
	return &TicketHandler{
		service: service,
		qr:      qr,
	}
}

func (h *TicketHandler) Join(
	c *gin.Context,
) {

	ctx := c.Request.Context()
	userID := c.GetInt64("user_id")

	token := tokenFrom(c)
	if token == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	queue, err := h.qr.ResolveJoinToken(
		ctx,
		token,
	)
	if err != nil {
		h.respondJoinTokenError(c, err)
		return
	}

	ticket, err := h.service.JoinQueue(
		ctx,
		queue.ID,
		userID,
	)
	if err != nil {
		h.respondJoinError(c, err)
		return
	}

	respondData(
		c,
		http.StatusOK,
		dto.NewTicketResponse(ticket),
	)
}

// tokenFrom: token join dari query `t` (mis. GET /api/v1/join/preview?t=...) atau JSON body (POST /api/v1/join {t}).
func tokenFrom(c *gin.Context) string {
	if t := c.Query("t"); t != "" {
		return t
	}
	var req dto.JoinByTokenRequest
	if err := c.ShouldBindJSON(&req); err == nil {
		return req.Token
	}
	return ""
}

// Preview: GET /api/v1/join/preview?t=... — info publik antrian dari token join.
func (h *TicketHandler) Preview(
	c *gin.Context,
) {

	ctx := c.Request.Context()

	token := tokenFrom(c)
	if token == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	queue, err := h.qr.ResolveJoinToken(
		ctx,
		token,
	)
	if err != nil {
		h.respondJoinTokenError(c, err)
		return
	}

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
	switch {
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		return
	case errors.Is(err, apperror.ErrQRInvalid):
		respondError(c, http.StatusNotFound, "QR_INVALID", "invalid or revoked qr token")
		return
	case errors.Is(err, apperror.ErrQRExpired):
		respondError(c, http.StatusGone, "QR_EXPIRED", "qr token expired")
		return
	case errors.Is(err, apperror.ErrQRUnavailable):
		respondError(c, http.StatusServiceUnavailable, "QR_UNAVAILABLE", "qr service is unavailable")
		return
	}
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
	switch {
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		return
	case errors.Is(err, apperror.ErrAlreadyJoined):
		respondError(c, http.StatusConflict, "ALREADY_JOINED", "you have already joined this queue")
		return
	case errors.Is(err, apperror.ErrQueueClosed):
		respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		return
	case errors.Is(err, apperror.ErrRetryExhausted):
		respondError(c, http.StatusServiceUnavailable, "RETRY_EXHAUSTED", "failed to join queue after exhausting all retries")
		return
	}
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

	counterID, expected := counterActionBody(c)

	ticket, err := h.service.NextTicket(
		ctx,
		queueID,
		c.GetInt64("user_id"),
		counterID,
		expected,
	)
	if errors.Is(err, apperror.ErrCounterStateChanged) {
		respondError(c, http.StatusConflict, "COUNTER_STATE_CHANGED", "this counter was just advanced from another device")
		return
	}
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			respondError(
				c,
				http.StatusNotFound,
				"NOT_FOUND",
				"queue not found",
			)
			return
		}
		if errors.Is(err, apperror.ErrQueueEmpty) {
			respondError(
				c,
				http.StatusConflict,
				"QUEUE_EMPTY",
				"queue is currently empty",
			)
			return
		}
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

	respondData(
		c,
		http.StatusOK,
		dto.NewTicketResponse(ticket),
	)
}

// Skip: POST /api/v1/queues/:queue_id/skip — owner melewati nomor yang sedang dilayani
// dan langsung memanggil nomor berikutnya.
func (h *TicketHandler) Skip(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	counterID, expected := counterActionBody(c)
	ticket, err := h.service.SkipTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID, expected)
	if errors.Is(err, apperror.ErrCounterStateChanged) {
		respondError(c, http.StatusConflict, "COUNTER_STATE_CHANGED", "this counter was just advanced from another device")
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		case errors.Is(err, apperror.ErrNothingToSkip):
			respondError(c, http.StatusConflict, "NOTHING_TO_SKIP", "no ticket is being served")
		case errors.Is(err, apperror.ErrQueueEmpty):
			respondError(c, http.StatusConflict, "QUEUE_EMPTY", "queue is currently empty")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

// Cancel: POST /api/v1/queues/:queue_id/cancel — customer membatalkan nomornya sendiri (4.9).
// Tanpa tiket aktif, atau sudah tidak aktif: 200 dengan keadaan tiket terakhir (no-op).
func (h *TicketHandler) Cancel(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	ticket, err := h.service.CancelTicket(c.Request.Context(), queueID, c.GetInt64("user_id"))
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or ticket not found")
		case errors.Is(err, apperror.ErrQueueClosed):
			respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

type insertTicketRequest struct {
	Identifier string `json:"identifier"`
	Position   int    `json:"position"`
}

// Insert: POST /api/v1/queues/:queue_id/insert — owner menyisipkan akun terdaftar di posisi tertentu (US-10).
func (h *TicketHandler) Insert(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	var req insertTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" || req.Position < 1 {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "identifier is required and position must be at least 1")
		return
	}
	ticket, err := h.service.InsertTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), identifier, req.Position)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or account not found")
		case errors.Is(err, apperror.ErrAmbiguousAccount):
			respondError(c, http.StatusConflict, "AMBIGUOUS_ACCOUNT", "more than one account has this username; use the email instead")
		case errors.Is(err, apperror.ErrAlreadyJoined):
			respondError(c, http.StatusConflict, "ALREADY_JOINED", "this account already has an active ticket in the queue")
		case errors.Is(err, apperror.ErrQueueClosed):
			respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		case errors.Is(err, apperror.ErrInvalidPosition):
			respondError(c, http.StatusBadRequest, "BAD_REQUEST", "position must be at least 1")
		case errors.Is(err, apperror.ErrRetryExhausted):
			respondError(c, http.StatusServiceUnavailable, "RETRY_EXHAUSTED", "failed to insert after exhausting all retries")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	respondData(c, http.StatusCreated, dto.NewTicketResponse(ticket))
}

// Remove: POST /api/v1/queues/:queue_id/tickets/:ticket_id/remove — owner menghapus nomor customer (4.10).
// Tiket sudah tidak aktif: 200 dengan keadaan tiket sekarang (no-op).
func (h *TicketHandler) Remove(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	ticketID, err := strconv.ParseInt(c.Param("ticket_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid ticket id")
		return
	}
	ticket, err := h.service.RemoveTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), ticketID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or ticket not found")
		case errors.Is(err, apperror.ErrQueueClosed):
			respondError(c, http.StatusConflict, "QUEUE_CLOSED", "queue is closed")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

// Recall: POST /api/v1/queues/:queue_id/recall — owner memanggil ulang nomor yang sedang dilayani (push saja).
func (h *TicketHandler) Recall(c *gin.Context) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}
	counterID, expected := counterActionBody(c)
	ticket, err := h.service.RecallTicket(c.Request.Context(), queueID, c.GetInt64("user_id"), counterID, expected)
	if errors.Is(err, apperror.ErrCounterStateChanged) {
		respondError(c, http.StatusConflict, "COUNTER_STATE_CHANGED", "this counter was just advanced from another device")
		return
	}
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrNotFound):
			respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		case errors.Is(err, apperror.ErrNothingToRecall):
			respondError(c, http.StatusConflict, "NOTHING_TO_RECALL", "no ticket is being served")
		case errors.Is(err, apperror.ErrRecallTooSoon):
			respondError(c, http.StatusConflict, "RECALL_TOO_SOON", "recall is available 3 seconds after calling")
		case errors.Is(err, apperror.ErrAlreadyRecalled):
			respondError(c, http.StatusConflict, "ALREADY_RECALLED", "this number was already recalled")
		default:
			respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}
	respondData(c, http.StatusOK, dto.NewTicketResponse(ticket))
}

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
	var statuses []string
	for _, raw := range c.QueryArray("status") {
		for _, st := range strings.Split(raw, ",") {
			st = strings.TrimSpace(st)
			if st == "" {
				continue
			}
			if !validTicketStatuses[st] {
				return nil, false
			}
			statuses = append(statuses, st)
		}
	}
	return statuses, true
}

func (h *TicketHandler) List(
	c *gin.Context,
) {

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

	requesterID := c.GetInt64("user_id")

	tickets, err := h.service.ListTickets(
		ctx,
		queueID,
		requesterID,
		statuses,
	)
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

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

func (h *TicketHandler) Status(
	c *gin.Context,
) {

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

	status, err := h.service.QueueStatus(
		c.Request.Context(),
		queueID,
		c.GetInt64("user_id"),
	)
	if errors.Is(err, apperror.ErrNotFound) {
		respondError(
			c,
			http.StatusNotFound,
			"NOT_FOUND",
			"queue not found",
		)
		return
	}
	if err != nil {
		respondError(
			c,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"internal server error",
		)
		return
	}

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
	var req dto.NextTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return 0, nil
	}
	return req.CounterID, req.ExpectedTicketID
}
