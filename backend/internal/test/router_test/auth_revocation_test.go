package router_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"qms/internal/apperror"
	"qms/internal/middleware"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

type fakeChecker struct {
	changedAt *time.Time
	err       error
	calls     int
}

func (f *fakeChecker) PasswordChangedAt(context.Context, int64) (*time.Time, error) {
	f.calls++
	return f.changedAt, f.err
}

func revocationProbe(checker middleware.SessionRevocationChecker, claims jwt.MapClaims) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/x", middleware.RequireAuthWithRevocation(testVerifier(""), checker), func(c *gin.Context) { c.Status(http.StatusOK) })
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(testKey)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	e.ServeHTTP(w, req)
	return w
}

func TestRevocation_TokenIssuedBeforePasswordChangeIs401(t *testing.T) {
	changed := time.Now()
	c := &fakeChecker{changedAt: &changed}
	w := revocationProbe(c, jwt.MapClaims{"sub": "5", "iat": changed.Add(-time.Minute).Unix(), "exp": changed.Add(time.Hour).Unix()})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRevocation_TokenIssuedAfterOrSameSecondIsAccepted(t *testing.T) {
	// Password diganti 2 menit lalu; pustaka JWT menolak iat di masa depan, jadi "sesudah" = 1 menit lalu.
	changed := time.Now().Add(-2 * time.Minute)
	for name, iat := range map[string]int64{"sesudah": changed.Add(time.Minute).Unix(), "detik sama": changed.Unix()} {
		c := &fakeChecker{changedAt: &changed}
		w := revocationProbe(c, jwt.MapClaims{"sub": "5", "iat": iat, "exp": changed.Add(time.Hour).Unix()})
		assert.Equal(t, http.StatusOK, w.Code, name)
	}
}

func TestRevocation_NeverChangedPasswordIsAccepted(t *testing.T) {
	c := &fakeChecker{}
	w := revocationProbe(c, jwt.MapClaims{"sub": "5", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, c.calls)
}

func TestRevocation_LegacyTokenWithoutIatSkipsCheck(t *testing.T) {
	changed := time.Now()
	c := &fakeChecker{changedAt: &changed}
	w := revocationProbe(c, jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()})
	assert.Equal(t, http.StatusOK, w.Code, "token lama tanpa iat tetap sah sampai exp")
	assert.Equal(t, 0, c.calls)
}

func TestRevocation_UnknownUserIs401AndDBErrorIs500(t *testing.T) {
	claims := jwt.MapClaims{"sub": "5", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	assert.Equal(t, http.StatusUnauthorized, revocationProbe(&fakeChecker{err: apperror.ErrNotFound}, claims).Code)
	assert.Equal(t, http.StatusInternalServerError, revocationProbe(&fakeChecker{err: errors.New("db down")}, claims).Code)
}
