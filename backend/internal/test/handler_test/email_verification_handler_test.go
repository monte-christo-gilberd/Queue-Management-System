package handler_test

import (
	"context"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubVerifySvc struct {
	verifyErr error
	sendErr   error
	gotToken  string
	gotUser   int64
	gotIP     string
	sent      int
}

func (s *stubVerifySvc) SendVerification(_ context.Context, userID int64, ip string) error {
	s.gotUser, s.gotIP = userID, ip
	s.sent++
	return s.sendErr
}

func (s *stubVerifySvc) Verify(_ context.Context, token string) error {
	s.gotToken = token
	return s.verifyErr
}

func TestVerifyEmail_Outcomes(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"sukses", nil, http.StatusOK, ""},
		{"token tidak sah", apperror.ErrInvalidResetToken, http.StatusBadRequest, "INVALID_TOKEN"},
		{"galat lain", assert.AnError, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, c := range cases {
		svc := &stubVerifySvc{verifyErr: c.err}
		e := specEngine(0)
		e.POST("/v", handler.NewEmailVerificationHandler(svc).Verify)

		w := specRequest(e, http.MethodPost, "/v", `{"token":"tok123"}`)

		assert.Equal(t, c.status, w.Code, c.name)
		if c.code != "" {
			assert.Equal(t, c.code, specErrorCode(t, w), c.name)
		}
		assert.Equal(t, "tok123", svc.gotToken, c.name)
		if c.err == nil {
			assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		}
	}
}

func TestVerifyEmail_InvalidBodyIs400AndSkipsService(t *testing.T) {
	svc := &stubVerifySvc{}
	e := specEngine(0)
	e.POST("/v", handler.NewEmailVerificationHandler(svc).Verify)

	for name, body := range map[string]string{
		"kosong":          `{}`,
		"bukan json":      `x`,
		"terlalu panjang": `{"token":"` + strings.Repeat("a", 201) + `"}`,
	} {
		w := specRequest(e, http.MethodPost, "/v", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
	assert.Equal(t, "", svc.gotToken)
}

func TestResendVerification_Outcomes(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"terkirim", nil, http.StatusAccepted, ""},
		{"batas per jam", apperror.ErrTooManyVerifyEmails, http.StatusTooManyRequests, "TOO_MANY_REQUESTS"},
		{"akun tidak ada", apperror.ErrNotFound, http.StatusUnauthorized, "UNAUTHORIZED"},
		{"galat lain", assert.AnError, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, c := range cases {
		svc := &stubVerifySvc{sendErr: c.err}
		e := specEngine(42)
		e.POST("/r", handler.NewEmailVerificationHandler(svc).Resend)

		w := specRequest(e, http.MethodPost, "/r", "")

		assert.Equal(t, c.status, w.Code, c.name)
		if c.code != "" {
			assert.Equal(t, c.code, specErrorCode(t, w), c.name)
		}
		assert.Equal(t, int64(42), svc.gotUser, "memakai user dari token login, bukan dari body")
		assert.NotEmpty(t, svc.gotIP)
	}
}

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
