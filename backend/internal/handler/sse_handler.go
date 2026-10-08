package handler

import (
	"context"
	"errors"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/sse"
	"strconv"

	"github.com/gin-gonic/gin"
)

type QueueValidator interface {
	GetQueue(
		ctx context.Context,
		queueID,
		requesterID int64,
	) (model.Queue, model.QueueAuthz, error)
}

type SSEHandler struct {
	hub    *sse.Hub
	queues QueueValidator
}

func NewSSEHandler(
	hub *sse.Hub,
	queues QueueValidator,
) *SSEHandler {
	return &SSEHandler{
		hub:    hub,
		queues: queues,
	}
}

func (h *SSEHandler) ServeSSE(
	c *gin.Context,
) {

	queueIDStr := c.Param("queue_id")
	requesterID := c.GetInt64("user_id")

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

	if h.queues != nil {
		
		if _, _, err := h.queues.GetQueue(
			c.Request.Context(),
			queueID,
			requesterID,
		); err != nil {
			if errors.Is(err, apperror.ErrNotFound) {
				respondError(
					c,
					http.StatusNotFound,
					"NOT_FOUND",
					"queue not found",
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
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	client := sse.NewClient(
		h.hub,
		c.Writer,
		queueID,
	)
	ok := h.hub.Subscribe(client)
	if !ok {
		respondError(
			c,
			http.StatusServiceUnavailable,
			"SERVICE_UNAVAILABLE",
			"service is not available",
		)
		return
	}
	client.Serve(c.Request)
}
