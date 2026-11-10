package httpapi_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"

	"qms/payment-service/internal/billing"
	"qms/payment-service/internal/httpapi"
)

// jwtSecret: secret HS256 lama. Hanya sah bila diberikan sebagai secret transisi ke httpapi.New.
const jwtSecret = "0123456789abcdef0123456789abcdef"

// testKey: kunci RSA yang menandatangani token seperti auth-service; API hanya memegang kunci publiknya.
var testKey = func() *rsa.PrivateKey {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return k
}()

type fakeBilling struct {
	healthErr   error
	checkout    billing.Checkout
	checkoutErr error
	gotOwner    int64
	webhookErr  error
	gotPayload  string
}

func (f *fakeBilling) Health(context.Context) error { return f.healthErr }
func (f *fakeBilling) StartCheckout(_ context.Context, id int64) (billing.Checkout, error) {
	f.gotOwner = id
	return f.checkout, f.checkoutErr
}
func (f *fakeBilling) HandleWebhook(_ context.Context, p []byte, _ string) error {
	f.gotPayload = string(p)
	return f.webhookErr
}

// handler membuat API tanpa secret transisi: hanya token RS256 yang diterima.
func handler(f *fakeBilling) http.Handler {
	return httpapi.New(f, &testKey.PublicKey, "", slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
}

// legacyHandler membuat API dengan secret transisi: token HS256 lama juga diterima.
func legacyHandler(f *fakeBilling) http.Handler {
	return httpapi.New(f, &testKey.PublicKey, jwtSecret, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
}

// rsaToken menandatangani klaim dengan kunci privat tes (RS256), seperti token login dari auth-service.
func rsaToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(testKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func token(t *testing.T, method jwt.SigningMethod, secret string, claims jwt.MapClaims) string {
	t.Helper()
	var key any = []byte(secret)
	if method == jwt.SigningMethodNone {
		key = jwt.UnsafeAllowNoneSignatureType
	}
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validToken(t *testing.T) string {
	return rsaToken(t, jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()})
}

func do(h http.Handler, method, path, auth, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth != "" {
		r.Header.Set("Authorization", "Bearer "+auth)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestCheckoutRequiresValidToken(t *testing.T) {
	cases := map[string]string{
		"none":      "",
		"garbage":   "abc.def.ghi",
		"wrong key": token(t, jwt.SigningMethodHS256, "another-secret-another-secret-123456", jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()}),
		"expired":   rsaToken(t, jwt.MapClaims{"sub": "7", "exp": time.Now().Add(-time.Hour).Unix()}),
		"alg none":  token(t, jwt.SigningMethodNone, "", jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()}),
		"bad sub":   rsaToken(t, jwt.MapClaims{"sub": "abc", "exp": time.Now().Add(time.Hour).Unix()}),
		"no sub":    rsaToken(t, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}),
		// HS256 lama ditolak setelah masa transisi (tanpa secret lama).
		"legacy hs256 after transition": token(t, jwt.SigningMethodHS256, jwtSecret, jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()}),
	}
	for name, tok := range cases {
		f := &fakeBilling{}
		w := do(handler(f), "POST", "/api/v1/payments/checkout", tok, "")
		if w.Code != http.StatusUnauthorized || f.gotOwner != 0 {
			t.Errorf("%s: code=%d owner=%d, want 401 and no call", name, w.Code, f.gotOwner)
		}
	}
}

func TestLegacyHS256AcceptedOnlyDuringTransition(t *testing.T) {
	// Selama transisi, token HS256 terbitan backend lama tetap sah; secret lain tetap ditolak.
	claims := jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()}
	f := &fakeBilling{checkout: billing.Checkout{URL: "https://pay.example/x"}}
	w := do(legacyHandler(f), "POST", "/api/v1/payments/checkout", token(t, jwt.SigningMethodHS256, jwtSecret, claims), "")
	if w.Code != http.StatusCreated || f.gotOwner != 7 {
		t.Fatalf("legacy token: code=%d owner=%d", w.Code, f.gotOwner)
	}
	f = &fakeBilling{}
	w = do(legacyHandler(f), "POST", "/api/v1/payments/checkout", token(t, jwt.SigningMethodHS256, "another-secret-another-secret-123456", claims), "")
	if w.Code != http.StatusUnauthorized || f.gotOwner != 0 {
		t.Fatalf("foreign secret: code=%d owner=%d, want 401", w.Code, f.gotOwner)
	}
}

func TestAlgorithmConfusionRejected(t *testing.T) {
	// HS256 yang ditandatangani memakai KUNCI PUBLIK sebagai secret harus ditolak, dengan atau tanpa secret lama.
	pub, err := x509.MarshalPKIXPublicKey(&testKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	forged := token(t, jwt.SigningMethodHS256, string(pub), jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()})
	for name, h := range map[string]http.Handler{"no legacy": handler(&fakeBilling{}), "with legacy": legacyHandler(&fakeBilling{})} {
		if w := do(h, "POST", "/api/v1/payments/checkout", forged, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: code=%d, want 401", name, w.Code)
		}
	}
}

func TestEmptyLegacySecretNeverAcceptsForgedHS256(t *testing.T) {
	// Tanpa secret transisi, HS256 yang ditandatangani dengan secret KOSONG tetap harus ditolak (HMAC dengan kunci
	// kosong sah secara teknis, jadi API harus menolak HS256 sama sekali).
	forged := token(t, jwt.SigningMethodHS256, "", jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()})
	f := &fakeBilling{}

	w := do(handler(f), "POST", "/api/v1/payments/checkout", forged, "")

	if w.Code != http.StatusUnauthorized || f.gotOwner != 0 {
		t.Fatalf("code=%d owner=%d, want 401 and no call", w.Code, f.gotOwner)
	}
}

func TestCheckoutSuccess(t *testing.T) {
	f := &fakeBilling{checkout: billing.Checkout{URL: "https://pay.example/x", ExpiresAt: time.Unix(1893456000, 0)}}
	w := do(handler(f), "POST", "/api/v1/payments/checkout", validToken(t), "")
	if w.Code != http.StatusCreated || f.gotOwner != 7 {
		t.Fatalf("code=%d owner=%d body=%s", w.Code, f.gotOwner, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"data"`) || !strings.Contains(w.Body.String(), "https://pay.example/x") {
		t.Fatalf("body = %s", w.Body)
	}
}

func TestCheckoutErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
		key  string
	}{
		{billing.ErrUnknownUser, 403, "UNKNOWN_USER"},
		{billing.ErrDisabled, 503, "PAYMENT_UNAVAILABLE"},
		{errors.New("stripe 500"), 502, "PAYMENT_PROVIDER_ERROR"},
	}
	for _, c := range cases {
		w := do(handler(&fakeBilling{checkoutErr: c.err}), "POST", "/api/v1/payments/checkout", validToken(t), "")
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.key) {
			t.Errorf("%v: code=%d body=%s", c.err, w.Code, w.Body)
		}
	}
}

func TestWebhookMapping(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{nil, 200},
		{billing.ErrBadSignature, 400},
		{billing.ErrDisabled, 503},
		{errors.New("db down"), 500},
	}
	for _, c := range cases {
		f := &fakeBilling{webhookErr: c.err}
		w := do(handler(f), "POST", "/api/v1/payments/webhooks/stripe", "", `{"x":1}`)
		if w.Code != c.code {
			t.Errorf("%v: code=%d, want %d", c.err, w.Code, c.code)
		}
		if f.gotPayload != `{"x":1}` {
			t.Errorf("payload not passed through unchanged: %q", f.gotPayload)
		}
	}
}

func TestWebhookBodyLimit(t *testing.T) {
	f := &fakeBilling{}
	w := do(handler(f), "POST", "/api/v1/payments/webhooks/stripe", "", strings.Repeat("a", 2<<20))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 for oversize body", w.Code)
	}
}

func TestHealth(t *testing.T) {
	if w := do(handler(&fakeBilling{}), "GET", "/health", "", ""); w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	if w := do(handler(&fakeBilling{healthErr: errors.New("x")}), "GET", "/health", "", ""); w.Code != 503 {
		t.Fatalf("code = %d", w.Code)
	}
}
