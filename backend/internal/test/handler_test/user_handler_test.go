package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
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
	isOwner       bool
	profileErr    error
}

func (s *stubUserService) Register(context.Context, string, string, string) (string, error) {
	return "", nil
}

func (s *stubUserService) Profile(_ context.Context, userID int64) (model.User, bool, error) {
	return model.User{ID: userID, Email: "me@test.com", Username: "me"}, s.isOwner, s.profileErr
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

func TestMe_ReturnsProfileAndOwnerFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubUserService{isOwner: false}
	e := gin.New()
	e.GET("/me", func(c *gin.Context) { c.Set("user_id", int64(9)); handler.NewUserHandler(svc).Me(c) })

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"id":9,"email":"me@test.com","username":"me","is_owner":false}}`, w.Body.String())
}
