package router_test

import (
	"net/http"
	"net/http/httptest"
	"qms/internal/middleware"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

const authSecret = "auth-spec-secret"

// authProbe: rute uji yang mencatat user_id dan apakah handler dijalankan.
func authProbe(t *testing.T, header string) (*httptest.ResponseRecorder, bool, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	reached := false
	var gotID int64
	e := gin.New()
	e.GET("/api/v1/queues", middleware.RequireAuth(authSecret), func(c *gin.Context) {
		reached = true
		gotID = c.GetInt64("user_id")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/queues", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	e.ServeHTTP(w, req)
	return w, reached, gotID
}

func signedToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	assert.NoError(t, err)
	return s
}

func TestSpec_Auth_MissingTokenIs401(t *testing.T) {
	// TC-08-05: API antrian menolak permintaan tanpa token dengan 401.
	w, reached, _ := authProbe(t, "")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
	assert.Contains(t, w.Body.String(), "UNAUTHORIZED")
}

func TestSpec_Auth_NonBearerSchemeIs401(t *testing.T) {
	// TC-08-04: header autentikasi yang bukan Bearer token ditolak.
	w, reached, _ := authProbe(t, "Basic dXNlcjpwYXNz")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
}

func TestSpec_Auth_TamperedOrForeignTokenIs401(t *testing.T) {
	// TC-08-04: token tidak valid (secret berbeda atau rusak) ditolak.
	valid := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}
	cases := map[string]string{
		"garbage":      "Bearer not-a-jwt",
		"other secret": "Bearer " + signedToken(t, "lain", valid),
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			w, reached, _ := authProbe(t, header)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.False(t, reached)
		})
	}
}

func TestSpec_Auth_ExpiredTokenIs401(t *testing.T) {
	// TC-08-04: sesi yang sudah kedaluwarsa tidak boleh dipakai.
	expired := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(-time.Minute).Unix()}

	w, reached, _ := authProbe(t, "Bearer "+signedToken(t, authSecret, expired))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
}

func TestSpec_Auth_NonNumericSubjectIs401(t *testing.T) {
	// Identitas user harus berupa id numerik; subject lain ditolak.
	claims := jwt.MapClaims{"sub": "abc", "exp": time.Now().Add(time.Hour).Unix()}

	w, reached, _ := authProbe(t, "Bearer "+signedToken(t, authSecret, claims))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
}

func TestSpec_Auth_ValidTokenPassesWithUserID(t *testing.T) {
	// TC-01-10: sesi yang sah diteruskan ke handler dengan user_id pemiliknya.
	w, reached, gotID := authProbe(t, "Bearer "+issueTestToken(authSecret, 42))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, reached)
	assert.Equal(t, int64(42), gotID)
}
