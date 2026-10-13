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
	) (model.Queue, model.QueueAuthz, error)
	ViewQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, model.QueueAuthz, bool, error)
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
	UpdateAuthz(
		ctx context.Context,
		queueID,
		requesterID int64,
		queueAuthz model.QueueAuthz,
	) (model.QueueAuthz, error)
	CloseQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) error
}

type QueueHandler struct {
	service QueueService
}

func NewQueueHandler(
	service QueueService,
) *QueueHandler {
	return &QueueHandler{
		service: service,
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

	queue, queueAuthz, err := h.service.CreateQueue(
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
			"action":      "create_queue",
			"queue":       dto.NewQueueResponse(queue),
			"queue_authz": dto.NewQueueAuthzResponse(queueAuthz),
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

	queue, queueAuthz, isOwner, err := h.service.ViewQueue(
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
			"action":      "get_queue",
			"queue":       dto.NewQueueResponse(queue),
			"queue_authz": dto.NewQueueAuthzResponse(queueAuthz),
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

func (h *QueueHandler) UpdateAuthz(
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

	var req dto.UpdateAuthzRequest
	if err := c.BindJSON(&req); err != nil {
		respondError(
			c,
			http.StatusBadRequest,
			"BAD_REQUEST",
			"invalid request body",
		)
		return
	}

	queueAuthz := model.QueueAuthz{
		QueueID:           queueID,
		CanManageQueue:    req.CanManageQueue,
		CanManageCounters: req.CanManageCounters,
		CanManageQr:       req.CanManageQr,
		CanViewQueue:      req.CanViewQueue,
		CanManageDisplay:  req.CanManageDisplay,
		CanContinueQueue:  req.CanContinueQueue,
		CanNotifyCustomer: req.CanNotifyCustomer,
		CanManageTickets:  req.CanManageTickets,
		CanManageNotes:    req.CanManageNotes,
	}

	freshAuthz, err := h.service.UpdateAuthz(
		c.Request.Context(),
		queueID,
		requesterID,
		queueAuthz,
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
			"action":      "update_authz",
			"queue_id":    queueID,
			"queue_authz": dto.NewQueueAuthzResponse(freshAuthz),
			"status":      "success",
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
