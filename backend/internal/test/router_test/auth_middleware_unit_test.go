package router_test

import (
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"qms/internal/middleware"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

// x509MarshalPublic mengubah kunci publik menjadi bytes PKIX (bahan serangan algorithm confusion di tes).
func x509MarshalPublic(pub *rsa.PublicKey) ([]byte, error) { return x509.MarshalPKIXPublicKey(pub) }

// authProbe: rute uji yang mencatat user_id dan apakah handler dijalankan.
func authProbe(t *testing.T, header string) (*httptest.ResponseRecorder, bool, int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	reached := false
	var gotID int64
	e := gin.New()
	e.GET("/api/v1/queues", middleware.RequireAuth(testVerifier("")), func(c *gin.Context) {
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

// signedToken menandatangani klaim dengan HS256 (secret bebas); dipakai untuk token lama atau token palsu.
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
	// TC-08-04: token tidak valid (kunci berbeda atau rusak) ditolak.
	valid := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}
	otherKey := mustKey()
	foreign, err := jwt.NewWithClaims(jwt.SigningMethodRS256, valid).SignedString(otherKey)
	assert.NoError(t, err)
	cases := map[string]string{
		"garbage":           "Bearer not-a-jwt",
		"other secret":      "Bearer " + signedToken(t, "lain", valid),
		"other private key": "Bearer " + foreign,
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

	w, reached, _ := authProbe(t, "Bearer "+signRS256(t, expired))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
}

func TestSpec_Auth_NonNumericSubjectIs401(t *testing.T) {
	// Identitas user harus berupa id numerik; subject lain ditolak.
	claims := jwt.MapClaims{"sub": "abc", "exp": time.Now().Add(time.Hour).Unix()}

	w, reached, _ := authProbe(t, "Bearer "+signRS256(t, claims))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
}

func TestSpec_Auth_ValidTokenPassesWithUserID(t *testing.T) {
	// TC-01-10: sesi yang sah diteruskan ke handler dengan user_id pemiliknya.
	w, reached, gotID := authProbe(t, "Bearer "+issueTestToken(42))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, reached)
	assert.Equal(t, int64(42), gotID)
}

func TestAuth_HS256IsRejectedWithoutLegacySecret(t *testing.T) {
	// Setelah masa transisi (JWT_LEGACY_HS256_SECRET kosong), token HS256 lama tidak diterima lagi.
	claims := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}

	w, reached, _ := authProbe(t, "Bearer "+signedToken(t, "secret-lama", claims))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
}

func TestAuth_HS256IsAcceptedOnlyWithMatchingLegacySecret(t *testing.T) {
	// Selama transisi, token HS256 terbitan backend lama (secret yang sama) tetap sah sampai kedaluwarsa.
	claims := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}
	v := testVerifier("secret-lama")

	_, err := v.Verify(signedToken(t, "secret-lama", claims))
	assert.NoError(t, err)

	_, err = v.Verify(signedToken(t, "secret-lain", claims))
	assert.Error(t, err)
}

func TestAuth_AlgorithmConfusionIsRejected(t *testing.T) {
	// Serangan klasik: penyerang menandatangani HS256 memakai KUNCI PUBLIK sebagai secret. Verifier harus menolak,
	// karena RS256 hanya boleh diverifikasi dengan kunci publik, dan HS256 hanya dengan secret lama.
	pubDER, err := x509MarshalPublic(&testKey.PublicKey)
	assert.NoError(t, err)
	claims := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}

	forged := signedToken(t, string(pubDER), claims)

	_, verr := testVerifier("").Verify(forged)
	assert.Error(t, verr)
	// walau secret lama terisi, secret itu bukan kunci publik, jadi tetap ditolak
	_, verr = testVerifier("secret-lama").Verify(forged)
	assert.Error(t, verr)
}

func TestAuth_NoneAlgorithmIsRejected(t *testing.T) {
	// Token tanpa tanda tangan (alg=none) tidak boleh diterima.
	claims := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	assert.NoError(t, err)

	_, verr := testVerifier("secret-lama").Verify(none)

	assert.Error(t, verr)
}

func TestAuth_EmptyLegacySecretNeverAcceptsForgedHS256(t *testing.T) {
	// Setelah masa transisi JWT_LEGACY_HS256_SECRET kosong. Token HS256 yang ditandatangani dengan secret KOSONG
	// tidak boleh lolos (HMAC dengan kunci kosong sah secara teknis, jadi verifier harus menolak HS256 sama sekali).
	claims := jwt.MapClaims{"sub": "5", "exp": time.Now().Add(time.Hour).Unix()}
	forged := signedToken(t, "", claims)

	w, reached, _ := authProbe(t, "Bearer "+forged)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, reached)
	_, err := testVerifier("").Verify(forged)
	assert.Error(t, err)
}
