package google_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/google"
	"qms/auth-service/internal/test/testutil"
)

const clientID = "123-web.apps.googleusercontent.com"

// keyServer menyajikan JWKS seperti Google. Kunci dan header bisa diganti selama tes berjalan (rotasi).
type keyServer struct {
	*httptest.Server
	mu     sync.Mutex
	keys   map[string]*rsa.PublicKey
	hits   atomic.Int32
	status int
	cache  string
}

func newKeyServer(t *testing.T, keys map[string]*rsa.PublicKey) *keyServer {
	t.Helper()
	ks := &keyServer{keys: keys, status: http.StatusOK, cache: "public, max-age=3600"}
	ks.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ks.hits.Add(1)
		ks.mu.Lock()
		defer ks.mu.Unlock()
		if ks.status != http.StatusOK {
			http.Error(w, "boom", ks.status)
			return
		}
		type jwk struct{ Kid, Kty, Alg, N, E string }
		var out struct{ Keys []map[string]string }
		for kid, pub := range ks.keys {
			out.Keys = append(out.Keys, map[string]string{
				"kid": kid, "kty": "RSA", "alg": "RS256", "use": "sig",
				"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			})
		}
		if ks.cache != "" {
			w.Header().Set("Cache-Control", ks.cache)
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(ks.Close)
	return ks
}

func (ks *keyServer) set(f func()) { ks.mu.Lock(); defer ks.mu.Unlock(); f() }

// claims membuat klaim ID token Google yang sah; ubah lewat map override.
func claims(now time.Time, override map[string]any) jwt.MapClaims {
	c := jwt.MapClaims{
		"iss": "https://accounts.google.com", "aud": clientID, "sub": "1234567890",
		"email": "budi@gmail.com", "email_verified": true, "name": "Budi Santoso",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	for k, v := range override {
		if v == nil {
			delete(c, k)
		} else {
			c[k] = v
		}
	}
	return c
}

func sign(t *testing.T, key *rsa.PrivateKey, kid string, c jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	tok.Header["kid"] = kid
	s, err := tok.SignedString(key)
	require.NoError(t, err)
	return s
}

var ctx = context.Background()

func setup(t *testing.T) (*google.Verifier, *keyServer, *rsa.PrivateKey, *time.Time) {
	t.Helper()
	key := testutil.Key
	ks := newKeyServer(t, map[string]*rsa.PublicKey{"k1": &key.PublicKey})
	now := time.Now()
	clock := &now
	v := google.NewVerifier(ks.URL, []string{clientID, "other-android-client"}).WithClock(func() time.Time { return *clock })
	return v, ks, key, clock
}

func TestVerify_ValidTokenReturnsIdentity(t *testing.T) {
	v, _, key, now := setup(t)

	ident, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, nil)))

	require.NoError(t, err)
	assert.Equal(t, "1234567890", ident.Subject)
	assert.Equal(t, "budi@gmail.com", ident.Email)
	assert.Equal(t, "Budi Santoso", ident.Name)
}

func TestVerify_AcceptsBothIssuerFormsAndAnyConfiguredAudience(t *testing.T) {
	v, _, key, now := setup(t)
	for name, over := range map[string]map[string]any{
		"iss tanpa skema":       {"iss": "accounts.google.com"},
		"aud client Android":    {"aud": "other-android-client"},
		"aud berupa daftar":     {"aud": []any{"orang-lain", clientID}},
		"email_verified 'true'": {"email_verified": "true"},
		"tanpa nama (boleh)":    {"name": nil},
	} {
		_, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, over)))
		assert.NoError(t, err, name)
	}
}

func TestVerify_RejectsInvalidTokens(t *testing.T) {
	v, _, key, now := setup(t)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	hs, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims(*now, nil)).SignedString(pubDER)
	require.NoError(t, err)
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims(*now, nil)).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	noKid := jwt.NewWithClaims(jwt.SigningMethodRS256, claims(*now, nil))
	noKidStr, err := noKid.SignedString(key)
	require.NoError(t, err)

	cases := map[string]string{
		"sampah":                       "bukan-jwt",
		"kosong":                       "",
		"ditandatangani kunci lain":    sign(t, otherKey, "k1", claims(*now, nil)),
		"kid tak dikenal":              sign(t, key, "kid-asing", claims(*now, nil)),
		"tanpa kid":                    noKidStr,
		"algorithm confusion HS256":    hs,
		"alg none":                     none,
		"aud aplikasi lain":            sign(t, key, "k1", claims(*now, map[string]any{"aud": "client-orang-lain"})),
		"aud daftar tanpa client kita": sign(t, key, "k1", claims(*now, map[string]any{"aud": []any{"a", "b"}})),
		"aud hilang":                   sign(t, key, "k1", claims(*now, map[string]any{"aud": nil})),
		"issuer bukan Google":          sign(t, key, "k1", claims(*now, map[string]any{"iss": "https://evil.example"})),
		"issuer hilang":                sign(t, key, "k1", claims(*now, map[string]any{"iss": nil})),
		"sub hilang":                   sign(t, key, "k1", claims(*now, map[string]any{"sub": nil})),
		"email hilang":                 sign(t, key, "k1", claims(*now, map[string]any{"email": nil})),
		"exp hilang":                   sign(t, key, "k1", claims(*now, map[string]any{"exp": nil})),
		"kedaluwarsa jauh":             sign(t, key, "k1", claims(*now, map[string]any{"exp": now.Add(-time.Hour).Unix()})),
		"iat di masa depan jauh":       sign(t, key, "k1", claims(*now, map[string]any{"iat": now.Add(time.Hour).Unix()})),
	}
	for name, raw := range cases {
		ident, err := v.Verify(ctx, raw)
		assert.ErrorIs(t, err, apperror.ErrGoogleInvalidToken, name)
		assert.Empty(t, ident.Subject, name)
	}
}

func TestVerify_UnverifiedEmailIsDistinctError(t *testing.T) {
	v, _, key, now := setup(t)
	for name, val := range map[string]any{"false": false, "string false": "false", "hilang": nil, "angka": 1} {
		_, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, map[string]any{"email_verified": val})))
		assert.ErrorIs(t, err, apperror.ErrGoogleEmailNotVerified, name)
	}
}

func TestVerify_ToleratesSmallClockSkewButNotLargeOnes(t *testing.T) {
	v, _, key, now := setup(t)

	// jam server tertinggal 30 detik dari Google: iat tampak di masa depan, tetap diterima
	_, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, map[string]any{"iat": now.Add(30 * time.Second).Unix()})))
	assert.NoError(t, err)
	// token baru saja kedaluwarsa 30 detik lalu: masih dalam toleransi
	_, err = v.Verify(ctx, sign(t, key, "k1", claims(*now, map[string]any{"exp": now.Add(-30 * time.Second).Unix()})))
	assert.NoError(t, err)
	// lewat toleransi 60 detik: ditolak
	_, err = v.Verify(ctx, sign(t, key, "k1", claims(*now, map[string]any{"exp": now.Add(-2 * time.Minute).Unix()})))
	assert.ErrorIs(t, err, apperror.ErrGoogleInvalidToken)
	_, err = v.Verify(ctx, sign(t, key, "k1", claims(*now, map[string]any{"iat": now.Add(2 * time.Minute).Unix()})))
	assert.ErrorIs(t, err, apperror.ErrGoogleInvalidToken)
}

func TestVerify_NoConfiguredClientIDsRejectsEverything(t *testing.T) {
	ks := newKeyServer(t, map[string]*rsa.PublicKey{"k1": &testutil.Key.PublicKey})
	v := google.NewVerifier(ks.URL, []string{"", "  "})

	_, err := v.Verify(ctx, sign(t, testutil.Key, "k1", claims(time.Now(), nil)))

	assert.ErrorIs(t, err, apperror.ErrGoogleInvalidToken)
	assert.Zero(t, ks.hits.Load(), "tidak perlu mengambil kunci bila tidak ada audience yang sah")
}

func TestKeys_AreCachedAcrossVerifications(t *testing.T) {
	v, ks, key, now := setup(t)

	for i := 0; i < 5; i++ {
		_, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, nil)))
		require.NoError(t, err)
	}

	assert.Equal(t, int32(1), ks.hits.Load(), "kunci diambil sekali, lalu dari cache")
}

func TestKeys_RefetchedAfterCacheExpiryAndGoogleDownMeansUnavailableNotStaleKeys(t *testing.T) {
	v, ks, key, now := setup(t)
	_, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, nil)))
	require.NoError(t, err)

	// cache (max-age 3600) kedaluwarsa dan Google mati: lebih aman gagal daripada memakai kunci yang mungkin dicabut
	*now = now.Add(2 * time.Hour)
	ks.set(func() { ks.status = http.StatusInternalServerError })
	_, err = v.Verify(ctx, sign(t, key, "k1", claims(*now, nil)))
	assert.ErrorIs(t, err, apperror.ErrGoogleUnavailable)

	// Google pulih: verifikasi berjalan lagi
	ks.set(func() { ks.status = http.StatusOK })
	*now = now.Add(2 * time.Minute)
	_, err = v.Verify(ctx, sign(t, key, "k1", claims(*now, nil)))
	assert.NoError(t, err)
}

func TestKeys_RotationIsPickedUpButUnknownKidsAreThrottled(t *testing.T) {
	v, ks, key, now := setup(t)
	_, err := v.Verify(ctx, sign(t, key, "k1", claims(*now, nil)))
	require.NoError(t, err)
	require.Equal(t, int32(1), ks.hits.Load())

	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rotated := sign(t, newKey, "k2", claims(*now, nil))

	// kid baru muncul sebelum 1 menit sejak pengambilan terakhir: tidak mengambil ulang (anti-banjir ke Google)
	_, err = v.Verify(ctx, rotated)
	assert.ErrorIs(t, err, apperror.ErrGoogleInvalidToken)
	assert.Equal(t, int32(1), ks.hits.Load())

	// Google merotasi kunci; setelah lewat 1 menit, kid baru dikenali tanpa menunggu TTL cache
	ks.set(func() { ks.keys["k2"] = &newKey.PublicKey })
	*now = now.Add(61 * time.Second)
	_, err = v.Verify(ctx, sign(t, newKey, "k2", claims(*now, nil)))
	assert.NoError(t, err)
	assert.Equal(t, int32(2), ks.hits.Load())
}

func TestKeys_GarbageResponsesMeanUnavailable(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"500":         func(w http.ResponseWriter, r *http.Request) { http.Error(w, "x", 500) },
		"bukan json":  func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html>") },
		"tanpa kunci": func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"keys":[]}`) },
		"kunci rusak": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"keys":[{"kid":"k1","kty":"RSA","n":"!!","e":"AQAB"}]}`)
		},
		"bukan RSA": func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"keys":[{"kid":"k1","kty":"EC"}]}`) },
	} {
		srv := httptest.NewServer(handler)
		v := google.NewVerifier(srv.URL, []string{clientID})

		_, err := v.Verify(ctx, sign(t, testutil.Key, "k1", claims(time.Now(), nil)))

		assert.ErrorIs(t, err, apperror.ErrGoogleUnavailable, name)
		srv.Close()
	}
}

func TestKeys_UnreachableGoogleMeansUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // alamat tidak lagi melayani
	v := google.NewVerifier(url, []string{clientID})

	_, err := v.Verify(ctx, sign(t, testutil.Key, "k1", claims(time.Now(), nil)))

	assert.ErrorIs(t, err, apperror.ErrGoogleUnavailable)
}
