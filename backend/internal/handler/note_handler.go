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

type NoteService interface {
	GetNote(
		ctx context.Context,
		queueID,
		requesterID,
		ticketID int64,
	) (model.TicketNote, error)
	SaveNote(
		ctx context.Context,
		queueID,
		requesterID,
		ticketID int64,
		note string,
	) (model.UpsertedNote, error)
}

type NoteHandler struct {
	service NoteService
}

func NewNoteHandler(
	service NoteService,
) *NoteHandler {
	return &NoteHandler{
		service: service,
	}
}

type saveNoteRequest struct {
	Note string `json:"note"`
}

// Get: GET /api/v1/queues/:queue_id/tickets/:ticket_id/note — owner membaca catatan satu nomor.
func (h *NoteHandler) Get(c *gin.Context) {
	queueID, ticketID, ok := noteIDs(c)
	if !ok {
		return
	}
	note, err := h.service.GetNote(c.Request.Context(), queueID, c.GetInt64("user_id"), ticketID)
	if err != nil {
		respondNoteError(c, err)
		return
	}
	respondData(c, http.StatusOK, dto.NewNoteResponse(note))
}

// Save: PUT /api/v1/queues/:queue_id/tickets/:ticket_id/note — membuat (201) atau menimpa (200) catatan.
func (h *NoteHandler) Save(c *gin.Context) {
	queueID, ticketID, ok := noteIDs(c)
	if !ok {
		return
	}
	var req saveNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	saved, err := h.service.SaveNote(c.Request.Context(), queueID, c.GetInt64("user_id"), ticketID, req.Note)
	if err != nil {
		respondNoteError(c, err)
		return
	}
	status := http.StatusOK
	if saved.Created {
		status = http.StatusCreated
	}
	respondData(c, status, dto.NewNoteResponse(saved.TicketNote))
}

func noteIDs(c *gin.Context) (queueID, ticketID int64, ok bool) {
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return 0, 0, false
	}
	ticketID, err = strconv.ParseInt(c.Param("ticket_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid ticket id")
		return 0, 0, false
	}
	return queueID, ticketID, true
}

func respondNoteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or ticket not found")
	case errors.Is(err, apperror.ErrInvalidNote):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "note must be 1-1000 characters")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
