package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	noHTMLValidator "qms/internal/validator"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

// Sama seperti cmd/server: validator kustom harus terdaftar sebelum binding JSON.
var registerValidators sync.Once

type stubUserService struct {
	gotIdentifier string
	token         string
	err           error
}

func (s *stubUserService) Register(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (s *stubUserService) Login(_ context.Context, identifier, _ string) (string, error) {
	s.gotIdentifier = identifier
	return s.token, s.err
}

func postLogin(t *testing.T, svc *stubUserService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registerValidators.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			_ = v.RegisterValidation("no_html_tags", noHTMLValidator.NoHTMLTags)
		}
	})
	e := gin.New()
	e.POST("/login", handler.NewUserHandler(svc).Login)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, req)
	return w
}

func TestLogin_AcceptsIdentifierEmailOrUsername(t *testing.T) {
	for _, body := range []string{
		`{"identifier":"budi@mail.com","password":"pw"}`,
		`{"identifier":"budi","password":"pw"}`,
	} {
		svc := &stubUserService{token: "tok"}
		w := postLogin(t, svc, body)

		assert.Equal(t, http.StatusOK, w.Code, body)
		assert.Contains(t, w.Body.String(), `"token":"tok"`)
	}
}

func TestLogin_EmailFieldStillWorks(t *testing.T) {
	svc := &stubUserService{token: "tok"}

	w := postLogin(t, svc, `{"email":"budi@mail.com","password":"pw"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "budi@mail.com", svc.gotIdentifier)
}

func TestLogin_IdentifierWinsOverEmail(t *testing.T) {
	svc := &stubUserService{token: "tok"}

	postLogin(t, svc, `{"identifier":"budi","email":"other@mail.com","password":"pw"}`)

	assert.Equal(t, "budi", svc.gotIdentifier)
}

func TestLogin_MissingIdentifierIsBadRequest(t *testing.T) {
	svc := &stubUserService{}

	w := postLogin(t, svc, `{"password":"pw"}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.gotIdentifier)
}

func TestLogin_WrongCredentialsIsUnauthorized(t *testing.T) {
	svc := &stubUserService{err: apperror.ErrNotFound}

	w := postLogin(t, svc, `{"identifier":"budi","password":"salah"}`)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), `"UNAUTHORIZED"`)
}
