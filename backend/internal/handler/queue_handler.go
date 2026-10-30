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

type QueueService interface {
	CreateQueue(
		ctx context.Context,
		ownerID int64,
		queueName,
		queueDescription string,
	) (model.Queue, error)
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, bool, error)
	ListQueue(
		ctx context.Context,
		requesterID int64,
	) ([]model.Queue, error)
	UpdateQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
		queueName,
		queueDescription string,
	) error
	CloseQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) error
}

type QueueQRService interface {
	GetQR(
		ctx context.Context,
		queueID int64,
		requesterID int64,
	) (dto.QueueQRResponse, error)
	SetQRType(
		ctx context.Context,
		queueID int64,
		requesterID int64,
		qrType string,
	) (dto.QueueQRResponse, error)
	ReplaceQR(
		ctx context.Context,
		queueID int64,
		requesterID int64,
	) (dto.QueueQRResponse, error)
}

type QueueHandler struct {
	service QueueService
	qr      QueueQRService
}

func NewQueueHandler(
	service QueueService,
) *QueueHandler {
	return &QueueHandler{
		service: service,
	}
}

func NewQueueHandlerWithQR(
	service QueueService,
	qr QueueQRService,
) *QueueHandler {
	return &QueueHandler{
		service: service,
		qr:      qr,
	}
}

func (h *QueueHandler) Create(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")

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

	queue, err := h.service.CreateQueue(
		c.Request.Context(),
		requesterID,
		req.QueueName,
		req.QueueDescription,
	)
	if errors.Is(err, apperror.ErrSubscriptionInactive) {
		respondError(
			c,
			http.StatusPaymentRequired,
			"SUBSCRIPTION_INACTIVE",
			"inactive owner subscription",
		)
		return
	}
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
		http.StatusCreated,
		gin.H{
			"action": "create_queue",
			"queue":  dto.NewQueueResponse(queue),
		},
	)
}

func (h *QueueHandler) Get(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")
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

	queue, isOwner, err := h.service.ViewQueue(
		c.Request.Context(),
		queueID,
		requesterID,
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

	// Pemegang tiket hanya melihat info publik queue (tanpa owner_id & izin).
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

	respondData(
		c,
		http.StatusOK,
		gin.H{
			"action": "get_queue",
			"queue":  dto.NewQueueResponse(queue),
		},
	)
}

func (h *QueueHandler) List(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")

	queues, err := h.service.ListQueue(
		c.Request.Context(),
		requesterID,
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
			"action": "list_queues",
			"queues": dto.NewQueueResponses(queues),
		},
	)
}

func (h *QueueHandler) Update(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")
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

	err = h.service.UpdateQueue(
		c.Request.Context(),
		queueID,
		requesterID,
		req.QueueName,
		req.QueueDescription,
	)
	if errors.Is(err, apperror.ErrSubscriptionInactive) {
		respondError(
			c,
			http.StatusPaymentRequired,
			"SUBSCRIPTION_INACTIVE",
			"inactive owner subscription",
		)
		return
	}
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
			"action":   "update_queue",
			"queue_id": queueID,
			"status":   "success",
		},
	)
}

func (h *QueueHandler) Close(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")
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

	err = h.service.CloseQueue(
		c.Request.Context(),
		queueID,
		requesterID,
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
	requesterID := c.GetInt64("user_id")
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}

	qr, err := h.qr.GetQR(c.Request.Context(), queueID, requesterID)
	if err != nil {
		h.respondQRError(c, err)
		return
	}

	respondData(c, http.StatusOK, qr)
}

// SetQRType: PUT /api/v1/queues/:queue_id/qr/type — set mode statis/dinamik.
func (h *QueueHandler) SetQRType(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}

	var req dto.SetQRTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	qr, err := h.qr.SetQRType(c.Request.Context(), queueID, requesterID, req.Type)
	if err != nil {
		h.respondQRError(c, err)
		return
	}

	respondData(c, http.StatusOK, qr)
}

// ReplaceQR: POST /api/v1/queues/:queue_id/qr/replace — mengganti kode statis.
func (h *QueueHandler) ReplaceQR(
	c *gin.Context,
) {
	requesterID := c.GetInt64("user_id")
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return
	}

	qr, err := h.qr.ReplaceQR(c.Request.Context(), queueID, requesterID)
	if err != nil {
		h.respondQRError(c, err)
		return
	}

	respondData(c, http.StatusOK, qr)
}

// respondQRError memetakan error QR ke response API.
func (h *QueueHandler) respondQRError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, apperror.ErrQRInvalid):
		respondError(c, http.StatusNotFound, "QR_INVALID", "invalid or revoked qr token")
		return
	case errors.Is(err, apperror.ErrQRExpired):
		respondError(c, http.StatusGone, "QR_EXPIRED", "qr token expired")
		return
	case errors.Is(err, apperror.ErrQRTypeInvalid):
		respondError(c, http.StatusBadRequest, "QR_TYPE_INVALID", "invalid qr type")
		return
	case errors.Is(err, apperror.ErrQRReplaceNotAllowed):
		respondError(c, http.StatusConflict, "QR_REPLACE_NOT_ALLOWED", "qr replacement is only allowed for static qr")
		return
	case errors.Is(err, apperror.ErrQRUnavailable):
		respondError(c, http.StatusServiceUnavailable, "QR_UNAVAILABLE", "qr service is unavailable")
		return
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}
}
