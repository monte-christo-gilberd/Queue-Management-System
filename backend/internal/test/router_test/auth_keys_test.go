package router_test

import (
	"crypto/rand"
	"crypto/rsa"
	"qms/internal/middleware"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
)

// testKey: kunci RSA yang dipakai tes untuk menandatangani token seperti auth-service. Backend hanya memegang
// kunci publiknya (lewat testVerifier).
var testKey = mustKey()

// mustKey membuat pasangan kunci RSA 2048 bit untuk tes; panic bila sumber keacakan bermasalah.
func mustKey() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
}

// testVerifier membuat TokenVerifier dengan kunci publik tes (hanya RS256 yang diterima).
func testVerifier() *middleware.TokenVerifier {
	return middleware.NewTokenVerifier(&testKey.PublicKey)
}

// signRS256 menandatangani klaim dengan kunci privat tes, seperti auth-service menerbitkan token login.
func signRS256(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(testKey)
	assert.NoError(t, err)
	return s
}

// issueTestToken menerbitkan token RS256 sah selama sejam untuk userID (dipakai tes rute).
func issueTestToken(userID int64) string {
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	s, _ := tok.SignedString(testKey)
	return s
}
