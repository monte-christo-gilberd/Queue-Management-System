package handler_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"
	"time"

	"qms/internal/apperror"
	"qms/internal/dto"
	"qms/internal/handler"
	"qms/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeNoteSvc: NoteService palsu yang merekam argumen dan mengembalikan nilai yang disiapkan.
type fakeNoteSvc struct {
	getNote    model.TicketNote
	getErr     error
	saved      model.UpsertedNote
	saveErr    error
	gotQueue   int64
	gotUser    int64
	gotTicket  int64
	gotNote    string
	getCalled  bool
	saveCalled bool
}

func (f *fakeNoteSvc) GetNote(_ context.Context, queueID, requesterID, ticketID int64) (model.TicketNote, error) {
	f.getCalled = true
	f.gotQueue, f.gotUser, f.gotTicket = queueID, requesterID, ticketID
	return f.getNote, f.getErr
}

func (f *fakeNoteSvc) SaveNote(_ context.Context, queueID, requesterID, ticketID int64, note string) (model.UpsertedNote, error) {
	f.saveCalled = true
	f.gotQueue, f.gotUser, f.gotTicket, f.gotNote = queueID, requesterID, ticketID, note
	return f.saved, f.saveErr
}

func noteEngine(svc *fakeNoteSvc, userID int64) *gin.Engine {
	e := specEngine(userID)
	h := handler.NewNoteHandler(svc)
	e.GET("/queues/:queue_id/tickets/:ticket_id/note", h.Get)
	e.PUT("/queues/:queue_id/tickets/:ticket_id/note", h.Save)
	return e
}

func TestNoteHandler_GetReturnsNoteForRequester(t *testing.T) {
	ts := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	svc := &fakeNoteSvc{getNote: model.TicketNote{
		TicketID:  5,
		Note:      sql.NullString{String: "minta struk", Valid: true},
		CreatedAt: sql.NullTime{Time: ts, Valid: true},
		UpdatedAt: sql.NullTime{Time: ts, Valid: true},
	}}
	e := noteEngine(svc, 9)

	w := specRequest(e, http.MethodGet, "/queues/3/tickets/5/note", "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(3), svc.gotQueue)
	assert.Equal(t, int64(9), svc.gotUser)
	assert.Equal(t, int64(5), svc.gotTicket)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, float64(5), data["ticket_id"])
	assert.Equal(t, "minta struk", data["note"])
	assert.NotNil(t, data["created_at"])
	assert.NotNil(t, data["updated_at"])
}

func TestNoteHandler_GetWithoutNoteReturnsNulls(t *testing.T) {
	svc := &fakeNoteSvc{getNote: model.TicketNote{TicketID: 5}}
	e := noteEngine(svc, 9)

	w := specRequest(e, http.MethodGet, "/queues/3/tickets/5/note", "")

	require.Equal(t, http.StatusOK, w.Code)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Nil(t, data["note"])
	assert.Nil(t, data["created_at"])
}

func TestNoteHandler_GetRejectsNonNumericIDs(t *testing.T) {
	svc := &fakeNoteSvc{}
	e := noteEngine(svc, 9)

	w := specRequest(e, http.MethodGet, "/queues/abc/tickets/5/note", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = specRequest(e, http.MethodGet, "/queues/3/tickets/xyz/note", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, svc.getCalled, "service tidak boleh dipanggil bila ID tidak valid")
}

func TestNoteHandler_GetMapsServiceErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound},
		{"unexpected", errors.New("db down"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := noteEngine(&fakeNoteSvc{getErr: tc.err}, 9)
			w := specRequest(e, http.MethodGet, "/queues/3/tickets/5/note", "")
			assert.Equal(t, tc.status, w.Code)
		})
	}
}

func TestNoteHandler_SaveCreatedReturns201(t *testing.T) {
	svc := &fakeNoteSvc{saved: model.UpsertedNote{
		TicketNote: model.TicketNote{
			TicketID: 5,
			Note:     sql.NullString{String: "baru", Valid: true},
		},
		Created: true,
	}}
	e := noteEngine(svc, 9)

	w := specRequest(e, http.MethodPut, "/queues/3/tickets/5/note", `{"note":"baru"}`)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.True(t, svc.saveCalled)
	assert.Equal(t, "baru", svc.gotNote)
	assert.Equal(t, int64(9), svc.gotUser)
	assert.Equal(t, "baru", specDecode(t, w)["data"].(map[string]interface{})["note"])
}

func TestNoteHandler_SaveOverwriteReturns200(t *testing.T) {
	svc := &fakeNoteSvc{saved: model.UpsertedNote{
		TicketNote: model.TicketNote{TicketID: 5, Note: sql.NullString{String: "ganti", Valid: true}},
		Created:    false,
	}}
	e := noteEngine(svc, 9)

	w := specRequest(e, http.MethodPut, "/queues/3/tickets/5/note", `{"note":"ganti"}`)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestNoteHandler_SaveRejectsBadInput(t *testing.T) {
	svc := &fakeNoteSvc{}
	e := noteEngine(svc, 9)

	w := specRequest(e, http.MethodPut, "/queues/x/tickets/5/note", `{"note":"a"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = specRequest(e, http.MethodPut, "/queues/3/tickets/y/note", `{"note":"a"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = specRequest(e, http.MethodPut, "/queues/3/tickets/5/note", `nope`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, svc.saveCalled)
}

func TestNoteHandler_SaveMapsServiceErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"not found", apperror.ErrNotFound, http.StatusNotFound},
		{"invalid note", apperror.ErrInvalidNote, http.StatusBadRequest},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := noteEngine(&fakeNoteSvc{saveErr: tc.err}, 9)
			w := specRequest(e, http.MethodPut, "/queues/3/tickets/5/note", `{"note":"a"}`)
			assert.Equal(t, tc.status, w.Code)
		})
	}
}

// fakeQueueQR: QueueQRService palsu untuk rute QR.
type fakeQueueQR struct {
	resp     dto.QueueQRResponse
	err      error
	gotType  string
	gotQueue int64
	gotUser  int64
	called   string
}

func (f *fakeQueueQR) GetQR(_ context.Context, queueID, requesterID int64) (dto.QueueQRResponse, error) {
	f.called, f.gotQueue, f.gotUser = "get", queueID, requesterID
	return f.resp, f.err
}

func (f *fakeQueueQR) SetQRType(_ context.Context, queueID, requesterID int64, qrType string) (dto.QueueQRResponse, error) {
	f.called, f.gotQueue, f.gotUser, f.gotType = "set", queueID, requesterID, qrType
	return f.resp, f.err
}

func (f *fakeQueueQR) ReplaceQR(_ context.Context, queueID, requesterID int64) (dto.QueueQRResponse, error) {
	f.called, f.gotQueue, f.gotUser = "replace", queueID, requesterID
	return f.resp, f.err
}

func qrEngine(qr *fakeQueueQR, userID int64) *gin.Engine {
	e := specEngine(userID)
	h := handler.NewQueueHandlerWithQR(newSpecQueueSvc(nil), qr)
	e.GET("/queues/:queue_id/qr", h.GetQR)
	e.PUT("/queues/:queue_id/qr/type", h.SetQRType)
	e.POST("/queues/:queue_id/qr/replace", h.ReplaceQR)
	return e
}

func sampleQR() dto.QueueQRResponse {
	exp := int64(1700000000)
	return dto.NewQueueQRResponse("queue_qr", 3, "dynamic", "https://example.test/join?t=v1", &exp)
}

func TestQueueQR_GetReturnsStatus(t *testing.T) {
	qr := &fakeQueueQR{resp: sampleQR()}
	e := qrEngine(qr, 7)

	w := specRequest(e, http.MethodGet, "/queues/3/qr", "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "get", qr.called)
	assert.Equal(t, int64(3), qr.gotQueue)
	assert.Equal(t, int64(7), qr.gotUser)
	data := specDecode(t, w)["data"].(map[string]interface{})
	assert.Equal(t, "dynamic", data["type"])
	assert.Equal(t, float64(1700000000), data["expires_at"])
}

func TestQueueQR_GetRejectsBadQueueID(t *testing.T) {
	qr := &fakeQueueQR{}
	e := qrEngine(qr, 7)

	w := specRequest(e, http.MethodGet, "/queues/abc/qr", "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, qr.called)
}

func TestQueueQR_SetTypePassesTypeToService(t *testing.T) {
	qr := &fakeQueueQR{resp: sampleQR()}
	e := qrEngine(qr, 7)

	w := specRequest(e, http.MethodPut, "/queues/3/qr/type", `{"type":"dynamic"}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "set", qr.called)
	assert.Equal(t, "dynamic", qr.gotType)
}

func TestQueueQR_SetTypeRejectsBadInput(t *testing.T) {
	qr := &fakeQueueQR{}
	e := qrEngine(qr, 7)

	assert.Equal(t, http.StatusBadRequest, specRequest(e, http.MethodPut, "/queues/x/qr/type", `{"type":"static"}`).Code)
	assert.Equal(t, http.StatusBadRequest, specRequest(e, http.MethodPut, "/queues/3/qr/type", `nope`).Code)
	assert.Empty(t, qr.called)
}

func TestQueueQR_ReplaceReturnsNewStatus(t *testing.T) {
	qr := &fakeQueueQR{resp: sampleQR()}
	e := qrEngine(qr, 7)

	w := specRequest(e, http.MethodPost, "/queues/3/qr/replace", "")

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "replace", qr.called)
	assert.Equal(t, int64(7), qr.gotUser)
}

func TestQueueQR_ReplaceRejectsBadQueueID(t *testing.T) {
	qr := &fakeQueueQR{}
	e := qrEngine(qr, 7)

	w := specRequest(e, http.MethodPost, "/queues/abc/qr/replace", "")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, qr.called)
}

func TestQueueQR_ErrorsMapToHTTPStatus(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"invalid token", apperror.ErrQRInvalid, http.StatusNotFound},
		{"expired token", apperror.ErrQRExpired, http.StatusGone},
		{"invalid type", apperror.ErrQRTypeInvalid, http.StatusBadRequest},
		{"replace not allowed", apperror.ErrQRReplaceNotAllowed, http.StatusConflict},
		{"qr unavailable", apperror.ErrQRUnavailable, http.StatusServiceUnavailable},
		{"queue not found", apperror.ErrNotFound, http.StatusNotFound},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qr := &fakeQueueQR{err: tc.err}
			e := qrEngine(qr, 7)

			assert.Equal(t, tc.status, specRequest(e, http.MethodGet, "/queues/3/qr", "").Code)
			assert.Equal(t, tc.status, specRequest(e, http.MethodPut, "/queues/3/qr/type", `{"type":"static"}`).Code)
			assert.Equal(t, tc.status, specRequest(e, http.MethodPost, "/queues/3/qr/replace", "").Code)
		})
	}
}
