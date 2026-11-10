package handler_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/handler"
	"qms/internal/model"
	noHTMLValidator "qms/internal/validator"
	"sync"
	"testing"
	"time"

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
	expiresAt     *time.Time
	gotUsername   string
	updateErr     error
	avatarErr     error
	avatar        model.Avatar
	avatarBytes   []byte
	deletedCalls  int
	avatarUpdated *time.Time
	emailOn       bool
	gotEmailPref  *bool
	prefErr       error
}

func (s *stubUserService) SetEmailNotifications(_ context.Context, _ int64, enabled bool) error {
	if s.prefErr != nil {
		return s.prefErr
	}
	s.gotEmailPref, s.emailOn = &enabled, enabled
	return nil
}

func (s *stubUserService) SetAvatar(_ context.Context, _ int64, r io.Reader) error {
	if s.avatarErr != nil {
		return s.avatarErr
	}
	s.avatarBytes, _ = io.ReadAll(r)
	return nil
}

func (s *stubUserService) Avatar(context.Context, int64) (model.Avatar, error) {
	return s.avatar, s.avatarErr
}

func (s *stubUserService) DeleteAvatar(context.Context, int64) error {
	s.deletedCalls++
	return s.avatarErr
}

func (s *stubUserService) Profile(_ context.Context, userID int64) (model.Profile, error) {
	return model.Profile{
		User:                  model.User{ID: userID, Email: "me@test.com", Username: "me", EmailNotifications: s.emailOn},
		IsOwner:               s.isOwner,
		SubscriptionExpiresAt: s.expiresAt,
		AvatarUpdatedAt:       s.avatarUpdated,
	}, s.profileErr
}

func (s *stubUserService) UpdateUsername(_ context.Context, userID int64, username string) (model.User, error) {
	s.gotUsername = username
	return model.User{ID: userID, Username: username}, s.updateErr
}

func TestMe_ReturnsProfileAndOwnerFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &stubUserService{isOwner: false}
	e := gin.New()
	e.GET("/me", func(c *gin.Context) { c.Set("user_id", int64(9)); handler.NewUserHandler(svc).Me(c) })

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"id":9,"email":"me@test.com","username":"me","is_owner":false,"subscription_expires_at":null,"avatar_updated_at":null,"email_notifications":false,"email_verified":false}}`, w.Body.String())
}

func TestMe_OwnerIncludesSubscriptionExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	exp := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	svc := &stubUserService{isOwner: true, expiresAt: &exp}
	e := gin.New()
	e.GET("/me", func(c *gin.Context) { c.Set("user_id", int64(9)); handler.NewUserHandler(svc).Me(c) })

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"data":{"id":9,"email":"me@test.com","username":"me","is_owner":true,"subscription_expires_at":"2026-12-01T00:00:00Z","avatar_updated_at":null,"email_notifications":false,"email_verified":false}}`, w.Body.String())
}

func putMe(t *testing.T, svc *stubUserService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	registerValidators.Do(func() {
		if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			_ = v.RegisterValidation("no_html_tags", noHTMLValidator.NoHTMLTags)
		}
	})
	e := gin.New()
	e.PUT("/me", func(c *gin.Context) { c.Set("user_id", int64(9)); handler.NewUserHandler(svc).UpdateMe(c) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/me", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, req)
	return w
}

func TestUpdateMe_ReturnsSameShapeAsMe(t *testing.T) {
	svc := &stubUserService{}
	w := putMe(t, svc, `{"username":"baru"}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "baru", svc.gotUsername)
	assert.Contains(t, w.Body.String(), `"subscription_expires_at":null`)
	assert.Contains(t, w.Body.String(), `"email":"me@test.com"`)
}

func TestUpdateMe_BadInputIsBadRequest(t *testing.T) {
	for name, body := range map[string]string{
		"missing":  `{}`,
		"empty":    `{"username":""}`,
		"html":     `{"username":"<b>x</b>"}`,
		"not json": `x`,
	} {
		svc := &stubUserService{}
		w := putMe(t, svc, body)
		assert.Equal(t, http.StatusBadRequest, w.Code, name)
		assert.Empty(t, svc.gotUsername, name)
	}
}

func TestUpdateMe_ServiceErrorsMapToStatus(t *testing.T) {
	w := putMe(t, &stubUserService{updateErr: apperror.ErrInvalidUsername}, `{"username":"x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = putMe(t, &stubUserService{updateErr: apperror.ErrNotFound}, `{"username":"x"}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	w = putMe(t, &stubUserService{updateErr: errors.New("boom")}, `{"username":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func avatarEngine(svc *stubUserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	h := handler.NewUserHandler(svc)
	e.Use(func(c *gin.Context) { c.Set("user_id", int64(9)) })
	e.PUT("/avatar", h.UploadAvatar)
	e.GET("/avatar", h.GetAvatar)
	e.DELETE("/avatar", h.DeleteAvatar)
	return e
}

func multipartBody(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if field != "" {
		part, err := w.CreateFormFile(field, filename)
		assert.NoError(t, err)
		_, _ = part.Write(content)
	}
	assert.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func putAvatar(e *gin.Engine, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/avatar", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestUploadAvatar_PassesFileAndReturnsProfile(t *testing.T) {
	svc := &stubUserService{}
	body, ct := multipartBody(t, "file", "a.jpg", []byte("isi-gambar"))

	w := putAvatar(avatarEngine(svc), body, ct)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []byte("isi-gambar"), svc.avatarBytes)
	assert.Contains(t, w.Body.String(), `"avatar_updated_at"`)
}

func TestUploadAvatar_MissingFileIsBadRequest(t *testing.T) {
	body, ct := multipartBody(t, "", "", nil)
	w := putAvatar(avatarEngine(&stubUserService{}), body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	body, ct = multipartBody(t, "bukan-file", "a.jpg", []byte("x"))
	w = putAvatar(avatarEngine(&stubUserService{}), body, ct)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadAvatar_ServiceErrorsMapToStatus(t *testing.T) {
	for err, want := range map[error]int{
		apperror.ErrImageTooLarge:    http.StatusRequestEntityTooLarge,
		apperror.ErrUnsupportedImage: http.StatusUnsupportedMediaType,
		apperror.ErrInvalidImage:     http.StatusBadRequest,
		errors.New("boom"):           http.StatusInternalServerError,
	} {
		body, ct := multipartBody(t, "file", "a.jpg", []byte("x"))
		w := putAvatar(avatarEngine(&stubUserService{avatarErr: err}), body, ct)
		assert.Equal(t, want, w.Code, err.Error())
	}
}

func TestUploadAvatar_OversizedRequestIs413(t *testing.T) {
	body, ct := multipartBody(t, "file", "a.jpg", bytes.Repeat([]byte("a"), 6<<20))
	w := putAvatar(avatarEngine(&stubUserService{}), body, ct)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestGetAvatar_ServesImageWithETagAnd304(t *testing.T) {
	updated := time.Date(2026, 11, 4, 1, 2, 3, 0, time.UTC)
	svc := &stubUserService{avatar: model.Avatar{Data: []byte("jpegbytes"), ContentType: "image/jpeg", UpdatedAt: updated}}
	e := avatarEngine(svc)

	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/avatar", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "image/jpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "private, no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "jpegbytes", w.Body.String())
	etag := w.Header().Get("ETag")
	assert.NotEmpty(t, etag)

	req := httptest.NewRequest(http.MethodGet, "/avatar", nil)
	req.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotModified, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestGetAvatar_NoneIs404(t *testing.T) {
	w := httptest.NewRecorder()
	avatarEngine(&stubUserService{avatarErr: apperror.ErrNotFound}).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/avatar", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteAvatar_Is204(t *testing.T) {
	svc := &stubUserService{}
	w := httptest.NewRecorder()
	avatarEngine(svc).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/avatar", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, 1, svc.deletedCalls)
}

func putPrefs(t *testing.T, svc *stubUserService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.PUT("/prefs", func(c *gin.Context) { c.Set("user_id", int64(9)); handler.NewUserHandler(svc).UpdatePreferences(c) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/prefs", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, req)
	return w
}

func TestUpdatePreferences_FalseIsAppliedAndEchoedInProfile(t *testing.T) {
	svc := &stubUserService{emailOn: true}
	w := putPrefs(t, svc, `{"email_notifications":false}`)
	assert.Equal(t, http.StatusOK, w.Code)
	if assert.NotNil(t, svc.gotEmailPref) {
		assert.False(t, *svc.gotEmailPref, "false harus dibedakan dari field hilang")
	}
	assert.Contains(t, w.Body.String(), `"email_notifications":false`)

	w = putPrefs(t, svc, `{"email_notifications":true}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"email_notifications":true`)
}

func TestUpdatePreferences_BadInputIsBadRequest(t *testing.T) {
	for name, body := range map[string]string{
		"missing": `{}`, "null": `{"email_notifications":null}`, "string": `{"email_notifications":"yes"}`, "not json": `x`,
	} {
		svc := &stubUserService{}
		assert.Equal(t, http.StatusBadRequest, putPrefs(t, svc, body).Code, name)
		assert.Nil(t, svc.gotEmailPref, name)
	}
}

func TestUpdatePreferences_ServiceErrorsMapToStatus(t *testing.T) {
	assert.Equal(t, http.StatusUnauthorized, putPrefs(t, &stubUserService{prefErr: apperror.ErrNotFound}, `{"email_notifications":true}`).Code)
	assert.Equal(t, http.StatusInternalServerError, putPrefs(t, &stubUserService{prefErr: errors.New("boom")}, `{"email_notifications":true}`).Code)
}
