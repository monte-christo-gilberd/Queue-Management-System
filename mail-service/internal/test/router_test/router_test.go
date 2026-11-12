package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"qms/mail-service/internal/handler"
	"qms/mail-service/internal/mailtemplate"
	"qms/mail-service/internal/middleware"
	"qms/mail-service/internal/model"
	"qms/mail-service/internal/router"
	"qms/mail-service/internal/service"
)

const token = "0123456789abcdef0123456789abcdef"

type fakeSender struct {
	mu    sync.Mutex
	sent  []model.Message
	err   error
	block chan struct{}
}

func (f *fakeSender) Send(ctx context.Context, m model.Message) error {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

func do(t *testing.T, h http.Handler, method, path, tok, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if tok != "" {
		req.Header.Set("X-Service-Token", tok)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// newAPI merangkai service, handler, middleware, dan router seperti di main.
func newAPI(s service.Sender) http.Handler {
	svc := service.NewMailService(s, mailtemplate.New(), nil)
	return router.SetupRouter(router.NewAppRouter(
		handler.NewMailHandler(svc),
		middleware.ServiceToken(token),
	).WithSystem(handler.NewSystemHandler()))
}

const okBody = `{"to":"budi@example.com","template":"test","data":{"To":"budi@example.com","SentAt":"4 Nov","Link":"http://x.test/a?b=1"}}`

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var r struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	return r.Error.Code
}

func TestHealth_NeedsNoTokenAndDoesNotTouchSMTP(t *testing.T) {
	f := &fakeSender{err: errors.New("smtp down")}
	w := do(t, newAPI(f), "GET", "/health", "", "")
	if w.Code != 200 || len(f.sent) != 0 {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestSend_RequiresServiceToken(t *testing.T) {
	f := &fakeSender{}
	for name, tok := range map[string]string{"tanpa": "", "salah": "x" + token[1:], "pendek": "abc"} {
		w := do(t, newAPI(f), "POST", "/internal/v1/send", tok, okBody)
		if w.Code != 401 || errCode(t, w) != "UNAUTHORIZED" {
			t.Errorf("%s: code=%d body=%s", name, w.Code, w.Body)
		}
	}
	if len(f.sent) != 0 {
		t.Fatal("nothing may be sent without a token")
	}
}

func TestSend_RendersTemplateAndSends(t *testing.T) {
	f := &fakeSender{}
	w := do(t, newAPI(f), "POST", "/internal/v1/send", token, okBody)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"sent"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	if len(f.sent) != 1 {
		t.Fatalf("sent=%d", len(f.sent))
	}
	m := f.sent[0]
	if m.To != "budi@example.com" || m.Subject != "Email uji dari Antrian" || !strings.Contains(m.Text, "http://x.test/a?b=1") || !strings.Contains(m.HTML, "<!doctype html>") {
		t.Fatalf("message %+v", m)
	}
}

func TestSend_RejectsBadRequestsWithoutSending(t *testing.T) {
	f := &fakeSender{}
	h := newAPI(f)
	cases := map[string]struct{ body, code string }{
		"json rusak":        {`{`, "BAD_REQUEST"},
		"field asing":       {`{"to":"a@b.co","template":"test","data":{},"subject":"curang"}`, "BAD_REQUEST"},
		"penerima kosong":   {`{"template":"test","data":{}}`, "BAD_REQUEST"},
		"penerima tak sah":  {`{"to":"bukan","template":"test","data":{}}`, "BAD_REQUEST"},
		"header injection":  {"{\"to\":\"a@b.co\\r\\nBcc: x@y.z\",\"template\":\"test\",\"data\":{}}", "BAD_REQUEST"},
		"templat tak ada":   {`{"to":"a@b.co","template":"tidak-ada","data":{}}`, "UNKNOWN_TEMPLATE"},
		"templat traversal": {`{"to":"a@b.co","template":"../x","data":{}}`, "UNKNOWN_TEMPLATE"},
		"data tak lengkap":  {`{"to":"a@b.co","template":"test","data":{"To":"a@b.co"}}`, "BAD_REQUEST"},
		"tanpa data":        {`{"to":"a@b.co","template":"test"}`, "BAD_REQUEST"},
	}
	for name, c := range cases {
		w := do(t, h, "POST", "/internal/v1/send", token, c.body)
		if w.Code != 400 || errCode(t, w) != c.code {
			t.Errorf("%s: code=%d want 400/%s body=%s", name, w.Code, c.code, w.Body)
		}
	}
	if len(f.sent) != 0 {
		t.Fatalf("sent=%d", len(f.sent))
	}
}

func TestSend_BodyTooLargeIs413(t *testing.T) {
	big := `{"to":"a@b.co","template":"test","data":{"x":"` + strings.Repeat("a", 70<<10) + `"}}`
	w := do(t, newAPI(&fakeSender{}), "POST", "/internal/v1/send", token, big)
	if w.Code != 413 || errCode(t, w) != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
}

func TestSend_SMTPFailureIs502AndDoesNotLeakDetails(t *testing.T) {
	f := &fakeSender{err: errors.New("dial tcp 10.1.2.3:587: secret-host connection refused")}
	w := do(t, newAPI(f), "POST", "/internal/v1/send", token, okBody)
	if w.Code != 502 || errCode(t, w) != "SMTP_FAILED" {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "10.1.2.3") || strings.Contains(w.Body.String(), "secret-host") {
		t.Fatalf("SMTP detail leaked: %s", w.Body)
	}
}

func TestSend_LimitsConcurrentSends(t *testing.T) {
	f := &fakeSender{block: make(chan struct{})}
	h := newAPI(f)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			do(t, h, "POST", "/internal/v1/send", token, okBody)
		}()
	}
	time.Sleep(300 * time.Millisecond) // beri waktu 16 permintaan tertahan di Send
	w := do(t, h, "POST", "/internal/v1/send", token, okBody)
	if w.Code != 503 || errCode(t, w) != "BUSY" {
		t.Fatalf("17th concurrent send: code=%d body=%s", w.Code, w.Body)
	}
	close(f.block)
	wg.Wait()
	if w := do(t, h, "POST", "/internal/v1/send", token, okBody); w.Code != 200 {
		t.Fatalf("after drain: code=%d", w.Code)
	}
}

func TestUnknownRouteIs404JSON(t *testing.T) {
	w := do(t, newAPI(&fakeSender{}), "GET", "/internal/v1/nope", token, "")
	if w.Code != 404 || errCode(t, w) != "NOT_FOUND" {
		t.Fatalf("code=%d body=%s", w.Code, w.Body)
	}
}

func TestResponseShapesAndHeaders(t *testing.T) {
	h := newAPI(&fakeSender{})
	w := do(t, h, "GET", "/health", "", "")
	if strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Errorf("health body = %s", w.Body)
	}
	w = do(t, h, "POST", "/internal/v1/send", token, okBody)
	if strings.TrimSpace(w.Body.String()) != `{"data":{"status":"sent"}}` {
		t.Errorf("send body = %s", w.Body)
	}
	// gin menambah "; charset=utf-8" pada Content-Type; klien tetap membaca JSON yang sama.
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("headers = %v", w.Header())
	}
	w = do(t, h, "GET", "/nope", token, "")
	if !strings.Contains(w.Body.String(), `"route not found"`) {
		t.Errorf("404 body = %s", w.Body)
	}
	w = do(t, h, "POST", "/internal/v1/send", "salah", okBody)
	if !strings.Contains(w.Body.String(), `"invalid service token"`) {
		t.Errorf("401 body = %s", w.Body)
	}
}
