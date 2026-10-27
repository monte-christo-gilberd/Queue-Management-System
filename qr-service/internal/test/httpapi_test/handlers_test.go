package httpapi_test

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qms/qr-service/internal/httpapi"
	"qms/qr-service/internal/joinlink"
)

const (
	testSecret       = "0123456789abcdef0123456789abcdef"
	testServiceToken = "service-token-for-tests"
	testPublicURL    = "https://qms.example.com"
)

// Bentuk body JSON di bawah ini adalah kontrak HTTP qr-service. Disalin ke sini
// supaya tes bersifat black-box dan tidak bergantung pada tipe internal handler.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

type tokenResponse struct {
	Token     string `json:"token"`
	Type      string `json:"type"`
	ExpiresAt *int64 `json:"expires_at"`
}

type verifyTokenRequest struct {
	Token string `json:"token"`
}

type verifyTokenResponse struct {
	Valid     bool   `json:"valid"`
	QueueID   int64  `json:"queue_id"`
	Type      string `json:"type"`
	ExpiresAt *int64 `json:"expires_at"`
}

type testEnv struct {
	router http.Handler
	signer *joinlink.Signer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	signer, err := joinlink.New(testSecret)
	if err != nil {
		t.Fatalf("joinlink.New: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := httpapi.NewRouter(httpapi.Config{
		Signer:       signer,
		PublicAppURL: testPublicURL,
		DynamicTTL:   2 * time.Minute,
		ServiceToken: testServiceToken,
		Logger:       logger,
	})

	return &testEnv{router: router, signer: signer}
}

func (e *testEnv) do(t *testing.T, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *testEnv) internal(t *testing.T, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, path, body, map[string]string{
		"X-Service-Token": testServiceToken,
	})
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return resp.Error
}

func TestHealthAndReady(t *testing.T) {
	env := newTestEnv(t)

	for _, path := range []string{"/health", "/ready"} {
		rec := env.do(t, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, rec.Code)
		}
	}
}

func TestInternal_RequiresServiceToken(t *testing.T) {
	env := newTestEnv(t)
	body := []byte(`{"queue_id":1,"type":"static"}`)

	rec := env.do(t, http.MethodPost, "/internal/v1/join-token", body, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	rec = env.do(t, http.MethodPost, "/internal/v1/join-token", body, map[string]string{
		"X-Service-Token": "wrong",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}
}

func TestIssueToken_Static(t *testing.T) {
	env := newTestEnv(t)

	rec := env.internal(t, "/internal/v1/join-token", []byte(`{"queue_id":12,"type":"static"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Type != "static" || resp.Token == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.ExpiresAt != nil {
		t.Fatalf("static token must not expire, got %d", *resp.ExpiresAt)
	}

	claims, err := env.signer.Verify(resp.Token)
	if err != nil {
		t.Fatalf("issued token does not verify: %v", err)
	}
	if claims.QueueID != 12 {
		t.Fatalf("expected queue 12, got %d", claims.QueueID)
	}
}

func TestIssueToken_Dynamic(t *testing.T) {
	env := newTestEnv(t)

	rec := env.internal(t, "/internal/v1/join-token", []byte(`{"queue_id":3,"type":"dynamic"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Type != "dynamic" || resp.ExpiresAt == nil {
		t.Fatalf("expected dynamic token with expiry, got %+v", resp)
	}
	if *resp.ExpiresAt <= time.Now().Unix() {
		t.Fatalf("expiry %d is not in the future", *resp.ExpiresAt)
	}
}

func TestIssueToken_Validation(t *testing.T) {
	env := newTestEnv(t)

	cases := []struct {
		name string
		body string
		code string
	}{
		{"invalid json", `not-json`, "BAD_REQUEST"},
		{"zero queue", `{"queue_id":0,"type":"static"}`, "BAD_REQUEST"},
		{"invalid type", `{"queue_id":1,"type":"forever"}`, "QR_TYPE_INVALID"},
		{"ttl out of range", `{"queue_id":1,"type":"dynamic","ttl_seconds":5}`, "BAD_REQUEST"},
	}

	for _, tc := range cases {
		rec := env.internal(t, "/internal/v1/join-token", []byte(tc.body))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", tc.name, rec.Code)
		}
		if got := decodeError(t, rec).Code; got != tc.code {
			t.Fatalf("%s: expected code %s, got %s", tc.name, tc.code, got)
		}
	}
}

func TestVerifyToken_Valid(t *testing.T) {
	env := newTestEnv(t)
	token, _, err := env.signer.Issue(9, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	body, _ := json.Marshal(verifyTokenRequest{Token: token})
	rec := env.internal(t, "/internal/v1/join-token/verify", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp verifyTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Valid || resp.QueueID != 9 || resp.Type != "static" || resp.ExpiresAt != nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestVerifyToken_Errors(t *testing.T) {
	env := newTestEnv(t)

	tampered, _, err := env.signer.Issue(9, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	parts := strings.Split(tampered, ".")
	if parts[2][0] == 'A' {
		parts[2] = "B" + parts[2][1:]
	} else {
		parts[2] = "A" + parts[2][1:]
	}
	tampered = strings.Join(parts, ".")

	expired, _, err := env.signer.Issue(9, joinlink.TypeDynamic, time.Millisecond)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	cases := []struct {
		name   string
		token  string
		status int
		code   string
	}{
		{"tampered", tampered, http.StatusUnauthorized, "QR_INVALID"},
		{"expired", expired, http.StatusGone, "QR_EXPIRED"},
		{"malformed", "garbage", http.StatusBadRequest, "QR_MALFORMED"},
		{"empty", "", http.StatusBadRequest, "QR_MALFORMED"},
	}

	for _, tc := range cases {
		body, _ := json.Marshal(verifyTokenRequest{Token: tc.token})
		rec := env.internal(t, "/internal/v1/join-token/verify", body)
		if rec.Code != tc.status {
			t.Fatalf("%s: expected %d, got %d (%s)", tc.name, tc.status, rec.Code, rec.Body.String())
		}
		if got := decodeError(t, rec).Code; got != tc.code {
			t.Fatalf("%s: expected code %s, got %s", tc.name, tc.code, got)
		}
	}
}

func TestJoinPNG_StaticIsCacheable(t *testing.T) {
	env := newTestEnv(t)
	token, _, err := env.signer.Issue(12, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/qr/join.png?token="+token+"&size=256", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("expected immutable cache header, got %q", cc)
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if cfg.Width != 256 {
		t.Fatalf("expected 256px, got %d", cfg.Width)
	}
}

func TestJoinPNG_DynamicIsNotCached(t *testing.T) {
	env := newTestEnv(t)
	token, _, err := env.signer.Issue(12, joinlink.TypeDynamic, time.Minute)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/qr/join.png?token="+token, nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("expected no-store, got %q", cc)
	}
}

func TestJoinPNG_RejectsBadTokens(t *testing.T) {
	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/qr/join.png", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing token: expected 400, got %d", rec.Code)
	}
	if got := decodeError(t, rec).Code; got != "QR_MALFORMED" {
		t.Fatalf("missing token: expected QR_MALFORMED, got %s", got)
	}

	rec = env.do(t, http.MethodGet, "/qr/join.png?token=garbage", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("garbage token: expected 400, got %d", rec.Code)
	}
}

func TestJoinPNG_RejectsBadRenderParams(t *testing.T) {
	env := newTestEnv(t)
	token, _, err := env.signer.Issue(1, joinlink.TypeStatic, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/qr/join.png?token="+token+"&size=large", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRenderGeneric(t *testing.T) {
	env := newTestEnv(t)

	rec := env.internal(t, "/internal/v1/qr/render", []byte(`{"data":"PAY-123","size":200}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
	if _, err := png.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Fatalf("decode png: %v", err)
	}

	rec = env.internal(t, "/internal/v1/qr/render", []byte(`{"data":""}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty data: expected 400, got %d", rec.Code)
	}
}

func TestInternal_BodyTooLarge(t *testing.T) {
	env := newTestEnv(t)
	body := []byte(`{"data":"` + strings.Repeat("a", 20<<10) + `"}`)

	rec := env.internal(t, "/internal/v1/qr/render", body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestUnknownRouteReturnsJSON(t *testing.T) {
	env := newTestEnv(t)

	rec := env.do(t, http.MethodGet, "/nope", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if got := decodeError(t, rec).Code; got != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %s", got)
	}
}
