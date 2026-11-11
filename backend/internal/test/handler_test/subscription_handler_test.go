package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"qms/internal/handler"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// stubSubscriptionService: merekam requesterID yang diterima dan mengembalikan expires atau err.
type stubSubscriptionService struct {
	expires   time.Time
	err       error
	gotUserID int64
}

func (s *stubSubscriptionService) Extend(_ context.Context, requesterID int64) (time.Time, error) {
	s.gotUserID = requesterID
	return s.expires, s.err
}

func newSubscriptionEngine(svc *stubSubscriptionService, userID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	// simulasikan middleware auth yang menaruh user_id dari token ke context
	e.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	})
	h := handler.NewSubscriptionHandler(svc)
	e.POST("/owners/subscription/extend", h.Extend)
	return e
}

func TestSubscription_ExtendOK(t *testing.T) {
	expires := time.Date(2026, 12, 11, 0, 0, 0, 0, time.UTC)
	svc := &stubSubscriptionService{expires: expires}
	w := httptest.NewRecorder()
	newSubscriptionEngine(svc, 42).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/owners/subscription/extend", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), svc.gotUserID, "user_id dari token harus diteruskan ke service")
	assert.Contains(t, w.Body.String(), `"subscription_expires_at":"2026-12-11T00:00:00Z"`)
}

func TestSubscription_ExtendDatabaseError(t *testing.T) {
	svc := &stubSubscriptionService{err: errors.New("db down")}
	w := httptest.NewRecorder()
	newSubscriptionEngine(svc, 42).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/owners/subscription/extend", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"INTERNAL_ERROR"`)
	assert.NotContains(t, w.Body.String(), "db down", "detail error database tidak boleh bocor ke klien")
}
