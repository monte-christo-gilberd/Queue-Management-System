package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"qms/internal/router"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type stubVerifyHandler struct{ verified, resent int }

func (s *stubVerifyHandler) Verify(c *gin.Context) { s.verified++; c.Status(http.StatusOK) }
func (s *stubVerifyHandler) Resend(c *gin.Context) { s.resent++; c.Status(http.StatusAccepted) }

func newVerifyEngine(h router.EmailVerificationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	app := router.NewAppRouter(struct{ router.TicketHandler }{}, struct{ router.UserHandler }{}, struct{ router.QueueHandler }{},
		func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) },
		stubSSE{}, nil)
	if h != nil {
		app.WithEmailVerification(h)
	}
	return router.SetupRouter(app)
}

func TestEmailVerificationRoutes_AbsentWithoutHandler(t *testing.T) {
	engine := newVerifyEngine(nil)

	for _, path := range []string{"/api/v1/users/verify-email", "/api/v1/users/me/verify-email/resend"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		assert.Equal(t, http.StatusNotFound, w.Code, path)
	}
}

func TestEmailVerificationRoutes_VerifyIsPublicAndResendNeedsLogin(t *testing.T) {
	h := &stubVerifyHandler{}
	engine := newVerifyEngine(h)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/users/verify-email", nil))
	assert.Equal(t, http.StatusOK, w.Code, "verifikasi memakai token di tautan, tanpa login")
	assert.Equal(t, 1, h.verified)

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/users/me/verify-email/resend", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code, "kirim ulang butuh login")
	assert.Equal(t, 0, h.resent)
}
