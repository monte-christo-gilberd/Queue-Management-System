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

// NoteService adalah logika catatan tiket (dipenuhi oleh service).
type NoteService interface {
	// GetNote mengambil catatan satu tiket, hanya untuk owner antrian.
	GetNote(
		ctx context.Context,
		queueID,
		requesterID,
		ticketID int64,
	) (model.TicketNote, error)
	// SaveNote menyimpan catatan tiket: membuat baru atau menimpa yang lama.
	SaveNote(
		ctx context.Context,
		queueID,
		requesterID,
		ticketID int64,
		note string,
	) (model.UpsertedNote, error)
}

// NoteHandler menangani rute catatan per nomor tiket.
type NoteHandler struct {
	// service berisi logika catatan tiket.
	service NoteService
}

// NewNoteHandler membuat NoteHandler dengan service yang diberikan.
func NewNoteHandler(
	service NoteService,
) *NoteHandler {
	// isi handler dengan service
	return &NoteHandler{
		service: service,
	}
}

// saveNoteRequest adalah body JSON untuk menyimpan catatan.
type saveNoteRequest struct {
	// Note adalah isi catatan yang akan disimpan.
	Note string `json:"note"`
}

// Get: GET /api/v1/queues/:queue_id/tickets/:ticket_id/note — owner membaca catatan satu nomor.
func (h *NoteHandler) Get(c *gin.Context) {
	// ambil dan periksa ID antrian serta ID tiket dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ticketID, ok := noteIDs(c)
	if !ok {
		return
	}
	// ambil catatan untuk user yang login; error diterjemahkan ke respons HTTP
	note, err := h.service.GetNote(c.Request.Context(), queueID, c.GetInt64("user_id"), ticketID)
	if err != nil {
		respondNoteError(c, err)
		return
	}
	// kirim catatan dalam bentuk respons DTO
	respondData(c, http.StatusOK, dto.NewNoteResponse(note))
}

// Save: PUT /api/v1/queues/:queue_id/tickets/:ticket_id/note — membuat (201) atau menimpa (200) catatan.
func (h *NoteHandler) Save(c *gin.Context) {
	// ambil dan periksa ID antrian serta ID tiket dari URL; bila tidak valid, respons error sudah dikirim
	queueID, ticketID, ok := noteIDs(c)
	if !ok {
		return
	}
	// baca body JSON yang berisi catatan; gagal berarti format request salah
	var req saveNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	// simpan catatan untuk user yang login; error diterjemahkan ke respons HTTP
	saved, err := h.service.SaveNote(c.Request.Context(), queueID, c.GetInt64("user_id"), ticketID, req.Note)
	if err != nil {
		respondNoteError(c, err)
		return
	}
	// status bawaan 200 OK (catatan ditimpa)
	status := http.StatusOK
	// catatan baru dibuat: ganti status menjadi 201 Created
	if saved.Created {
		status = http.StatusCreated
	}
	// kirim catatan yang tersimpan dengan status yang sudah ditentukan
	respondData(c, status, dto.NewNoteResponse(saved.TicketNote))
}

// noteIDs membaca dan memeriksa queue_id dan ticket_id dari URL. Bila salah satu bukan angka, ia mengirim
// respons 400 dan mengembalikan ok false.
func noteIDs(c *gin.Context) (queueID, ticketID int64, ok bool) {
	// ubah queue_id menjadi angka 64-bit; gagal berarti bukan angka
	queueID, err := strconv.ParseInt(c.Param("queue_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid queue id")
		return 0, 0, false
	}
	// ubah ticket_id menjadi angka 64-bit; gagal berarti bukan angka
	ticketID, err = strconv.ParseInt(c.Param("ticket_id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid ticket id")
		return 0, 0, false
	}
	// kedua ID valid
	return queueID, ticketID, true
}

// respondNoteError menerjemahkan error dari service catatan menjadi respons HTTP.
func respondNoteError(c *gin.Context, err error) {
	// pilih respons sesuai jenis error
	switch {
	// antrian atau tiket tidak ditemukan: 404
	case errors.Is(err, apperror.ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "queue or ticket not found")
	// isi catatan tidak memenuhi aturan panjang: 400
	case errors.Is(err, apperror.ErrInvalidNote):
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "note must be 1-1000 characters")
	// error lain: 500 tanpa detail
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	}
}
