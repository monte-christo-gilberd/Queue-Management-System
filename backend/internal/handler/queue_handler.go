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
	GetQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, model.QueueAuthz, error)
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
	) error
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
			http.StatusForbidden,
			"Forbidden",
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
			"queue":       queue,
			"queue_authz": queueAuthz,
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

	queue, queueAuthz, err := h.service.GetQueue(
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
			"action":      "get_queue",
			"queue":       queue,
			"queue_authz": queueAuthz,
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
			"queues": queues,
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
		QueueID:           req.QueueID,
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

	err = h.service.UpdateAuthz(
		c.Request.Context(),
		queueID,
		requesterID,
		queueAuthz,
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
			"action":   "update_authz",
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
