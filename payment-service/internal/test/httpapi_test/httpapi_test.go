package httpapi_test

import (
	"context"
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

const jwtSecret = "0123456789abcdef0123456789abcdef"

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

func handler(f *fakeBilling) http.Handler {
	return httpapi.New(f, jwtSecret, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
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
	return token(t, jwt.SigningMethodHS256, jwtSecret, jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()})
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
		"expired":   token(t, jwt.SigningMethodHS256, jwtSecret, jwt.MapClaims{"sub": "7", "exp": time.Now().Add(-time.Hour).Unix()}),
		"alg none":  token(t, jwt.SigningMethodNone, "", jwt.MapClaims{"sub": "7", "exp": time.Now().Add(time.Hour).Unix()}),
		"bad sub":   token(t, jwt.SigningMethodHS256, jwtSecret, jwt.MapClaims{"sub": "abc", "exp": time.Now().Add(time.Hour).Unix()}),
		"no sub":    token(t, jwt.SigningMethodHS256, jwtSecret, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()}),
	}
	for name, tok := range cases {
		f := &fakeBilling{}
		w := do(handler(f), "POST", "/api/v1/payments/checkout", tok, "")
		if w.Code != http.StatusUnauthorized || f.gotOwner != 0 {
			t.Errorf("%s: code=%d owner=%d, want 401 and no call", name, w.Code, f.gotOwner)
		}
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
		{billing.ErrNotOwner, 403, "NOT_OWNER"},
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
