package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/auth"
	"qms/auth-service/internal/test/testutil"
)

func newIssuer() *auth.TokenIssuer { return auth.NewTokenIssuer(testutil.Key, "kid-1") }

func TestIssue_ProducesRS256TokenWithSubIatExpAndKid(t *testing.T) {
	issuer := newIssuer()

	raw, err := issuer.Issue(42)
	require.NoError(t, err)

	parsed, err := jwt.Parse(raw, func(*jwt.Token) (interface{}, error) { return &testutil.Key.PublicKey, nil })
	require.NoError(t, err)
	assert.Equal(t, "RS256", parsed.Header["alg"])
	assert.Equal(t, "kid-1", parsed.Header["kid"])
	claims := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, strconv.FormatInt(42, 10), claims["sub"])
	iat, exp := int64(claims["iat"].(float64)), int64(claims["exp"].(float64))
	assert.InDelta(t, time.Now().Unix(), iat, 5, "iat = waktu terbit, dipakai pencabutan sesi")
	assert.Equal(t, 24*time.Hour, time.Duration(exp-iat)*time.Second, "berlaku 24 jam, sama dengan sebelum dipisah")
}

func TestVerify_AcceptsOwnTokenAndRejectsEverythingElse(t *testing.T) {
	issuer := newIssuer()
	good, err := issuer.Issue(7)
	require.NoError(t, err)
	valid := jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()}

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	foreign, err := jwt.NewWithClaims(jwt.SigningMethodRS256, valid).SignedString(otherKey)
	require.NoError(t, err)
	expired, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "7", "exp": time.Now().Add(-time.Minute).Unix()}).SignedString(testutil.Key)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&testutil.Key.PublicKey)
	require.NoError(t, err)
	// algorithm confusion: HS256 ditandatangani memakai kunci publik sebagai secret
	confused, err := jwt.NewWithClaims(jwt.SigningMethodHS256, valid).SignedString(pubDER)
	require.NoError(t, err)
	hs, err := jwt.NewWithClaims(jwt.SigningMethodHS256, valid).SignedString([]byte("secret-lama-yang-cukup-panjang-123456"))
	require.NoError(t, err)
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, valid).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	claims, err := issuer.Verify(good)
	require.NoError(t, err)
	assert.Equal(t, "7", claims["sub"])

	for name, raw := range map[string]string{
		"sampah":              "bukan-jwt",
		"kunci privat lain":   foreign,
		"kedaluwarsa":         expired,
		"algorithm confusion": confused,
		"HS256 secret lama":   hs,
		"alg none":            none,
	} {
		_, err := issuer.Verify(raw)
		assert.Error(t, err, name)
	}
}

func TestVerify_RejectsTamperedPayload(t *testing.T) {
	issuer := newIssuer()
	raw, err := issuer.Issue(7)
	require.NoError(t, err)

	// ganti satu karakter di bagian payload: tanda tangan tidak lagi cocok
	parts := []byte(raw)
	i := len(raw) / 2
	if parts[i] == 'A' {
		parts[i] = 'B'
	} else {
		parts[i] = 'A'
	}

	_, err = issuer.Verify(string(parts))
	assert.Error(t, err)
}
