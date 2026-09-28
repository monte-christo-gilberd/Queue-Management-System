package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/model"
	"strconv"

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
		queueID int64,
	) (model.Ticket, error)
}

type TicketHandler struct {
	service TicketService
}

func NewTicketHandler(service TicketService) *TicketHandler {
	return &TicketHandler{
		service: service,
	}
}

func (h *TicketHandler) Join(c *gin.Context) {

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

	userID := c.GetInt64("user_id")

	ticket, err := h.service.JoinQueue(
		ctx,
		queueID,
		userID,
	)
	if err != nil {
		if errors.Is(err, apperror.ErrRetryExhausted) {
			respondError(
				c,
				http.StatusServiceUnavailable,
				"RETRY_EXHAUSTED",
				"failed to join queue after exhauting all retries",
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
		ticket,
	)
}

func (h *TicketHandler) Next(c *gin.Context) {

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

	ticket, err := h.service.NextTicket(
		ctx,
		queueID,
	)
	if err != nil {
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
		ticket,
	)
}
