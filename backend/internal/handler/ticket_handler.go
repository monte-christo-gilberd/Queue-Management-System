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
		requesterID int64,
	) (model.Ticket, error)
	ListTickets(
		ctx context.Context,
		queueID,
		requesterID int64,
		statuses []string,
	) ([]model.Ticket, error)
}

type TicketHandler struct {
	service TicketService
}

func NewTicketHandler(
	service TicketService,
) *TicketHandler {
	return &TicketHandler{
		service: service,
	}
}

func (h *TicketHandler) Join(
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

	userID := c.GetInt64("user_id")

	ticket, err := h.service.JoinQueue(
		ctx,
		queueID,
		userID,
	)
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
		if errors.Is(err, apperror.ErrAlreadyJoined) {
			respondError(
				c,
				http.StatusConflict,
				"ALREADY_JOINED",
				"you have already joined this queue",
			)
			return
		}
		if errors.Is(err, apperror.ErrQueueClosed) {
			respondError(
				c,
				http.StatusConflict,
				"QUEUE_CLOSED",
				"queue is closed",
			)
			return
		}
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
		dto.NewTicketResponse(ticket),
	)
}

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

	ticket, err := h.service.NextTicket(
		ctx,
		queueID,
		c.GetInt64("user_id"),
	)
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

var validTicketStatuses = map[string]bool{
	"waiting":   true,
	"completed": true,
	"cancelled": true,
	"skipped":   true,
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
			"tickets":  dto.NewTicketResponses(tickets),
		},
	)
}
