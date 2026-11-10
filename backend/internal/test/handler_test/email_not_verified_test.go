package handler_test

import (
	"net/http"
	"qms/internal/apperror"
	"qms/internal/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Akun yang emailnya belum diverifikasi ditolak di buat antrian dan join dengan 403 EMAIL_NOT_VERIFIED.
// Verifikasi itu sendiri (kirim tautan, tandai terverifikasi) dilayani auth-service.

func TestCreateQueue_UnverifiedEmailIs403(t *testing.T) {
	svc := newSpecQueueSvc(t)
	svc.On("CreateQueue", specOwner, "Antrian", "").Return(model.Queue{}, apperror.ErrEmailNotVerified)
	e := specQueueEngine(svc, specOwner)

	w := specRequest(e, http.MethodPost, "/queues", `{"queue_name":"Antrian"}`)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "EMAIL_NOT_VERIFIED", specErrorCode(t, w))
	assert.NotContains(t, w.Body.String(), `"queue":`)
}

func TestJoin_UnverifiedEmailIs403WithoutTicket(t *testing.T) {
	svc := &specTicketSvc{
		joinQueue: model.Queue{ID: 1, OwnerID: 7, QueueName: "Loket Utama", Status: "active", CreatedAt: time.Now()},
		joinErr:   apperror.ErrEmailNotVerified,
	}
	e := specTicketEngine(svc, 42)

	w := specRequest(e, http.MethodPost, "/join", `{"t":"token-abc"}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "EMAIL_NOT_VERIFIED", specErrorCode(t, w))
	assert.NotContains(t, w.Body.String(), `"ticket_number"`)
}
