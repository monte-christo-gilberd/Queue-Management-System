package handler_test

import (
	"context"
	"net/http"
	"qms/internal/apperror"
	"qms/internal/handler"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubResetSvc struct {
	requested chan string
	resetErr  error
	gotToken  string
	gotPass   string
}

func (s *stubResetSvc) RequestReset(_ context.Context, email, _ string) error {
	s.requested <- email
	return nil
}

func (s *stubResetSvc) ResetPassword(_ context.Context, token, pass string) error {
	s.gotToken, s.gotPass = token, pass
	return s.resetErr
}

func TestForgotPassword_Always202AndSendsAsync(t *testing.T) {
	svc := &stubResetSvc{requested: make(chan string, 1)}
	e := specEngine(0)
	h := handler.NewPasswordHandler(svc)
	e.POST("/users/forgot-password", h.ForgotPassword)

	w := specRequest(e, http.MethodPost, "/users/forgot-password", `{"email":"budi@mail.com"}`)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), "Jika email terdaftar")
	select {
	case got := <-svc.requested:
		assert.Equal(t, "budi@mail.com", got)
	case <-time.After(2 * time.Second):
		t.Fatal("service tidak dipanggil")
	}
}

func TestForgotPassword_InvalidBodyIs400(t *testing.T) {
	svc := &stubResetSvc{requested: make(chan string, 1)}
	e := specEngine(0)
	e.POST("/f", handler.NewPasswordHandler(svc).ForgotPassword)

	for name, body := range map[string]string{
		"kosong":      `{}`,
		"bukan email": `{"email":"bukan-email"}`,
		"html":        `{"email":"<b>x</b>@a.com"}`,
		"bukan json":  `x`,
	} {
		w := specRequest(e, http.MethodPost, "/f", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
	}
	assert.Empty(t, svc.requested)
}

func TestResetPassword_Outcomes(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"sukses", nil, http.StatusOK, ""},
		{"token tidak sah", apperror.ErrInvalidResetToken, http.StatusBadRequest, "INVALID_TOKEN"},
		{"password lemah", apperror.ErrWeakPassword, http.StatusBadRequest, "WEAK_PASSWORD"},
		{"galat lain", assert.AnError, http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, c := range cases {
		svc := &stubResetSvc{requested: make(chan string, 1), resetErr: c.err}
		e := specEngine(0)
		e.POST("/r", handler.NewPasswordHandler(svc).ResetPassword)

		w := specRequest(e, http.MethodPost, "/r", `{"token":"tok123","password":"passwordbaru1"}`)

		assert.Equal(t, c.status, w.Code, c.name)
		if c.code != "" {
			assert.Equal(t, c.code, specErrorCode(t, w), c.name)
		}
		assert.Equal(t, "tok123", svc.gotToken)
		assert.Equal(t, "passwordbaru1", svc.gotPass)
	}
}

func TestResetPassword_MissingFieldsIs400(t *testing.T) {
	svc := &stubResetSvc{requested: make(chan string, 1)}
	e := specEngine(0)
	e.POST("/r", handler.NewPasswordHandler(svc).ResetPassword)
	for _, body := range []string{`{}`, `{"token":"x"}`, `{"password":"passwordbaru1"}`} {
		require.Equal(t, http.StatusBadRequest, specRequest(e, http.MethodPost, "/r", body).Code, body)
	}
	assert.Empty(t, svc.gotToken)
}
