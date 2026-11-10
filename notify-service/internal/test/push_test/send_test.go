package push_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"qms/notify-service/internal/message"
	"qms/notify-service/internal/push"
)

// testSub builds a subscription with a real P-256 key and auth secret so webpush can encrypt the payload.
func testSub(t *testing.T, endpoint string) message.Subscription {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return message.Subscription{
		Endpoint: endpoint,
		P256dh:   base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
	}
}

func newSender(t *testing.T) *push.Sender {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	return push.New(pub, priv, "mailto:test@example.com")
}

var testMsg = message.Message{Title: "Q", Body: "Giliran Anda!", URL: "/ticket/1"}

func TestSendDeliversEncryptedPush(t *testing.T) {
	var hits atomic.Int32
	var gotTTL, gotEnc, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		gotTTL = r.Header.Get("TTL")
		gotEnc = r.Header.Get("Content-Encoding")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	if err := newSender(t).Send(context.Background(), testSub(t, srv.URL), testMsg); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 || gotTTL != "60" || gotEnc != "aes128gcm" {
		t.Fatalf("hits=%d ttl=%q enc=%q", hits.Load(), gotTTL, gotEnc)
	}
	if len(gotAuth) < len("vapid ") || gotAuth[:6] != "vapid " {
		t.Fatalf("authorization=%q, want VAPID header", gotAuth)
	}
}

func TestSendGoneIsNotRetried(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	err := newSender(t).Send(context.Background(), testSub(t, srv.URL), testMsg)
	if !errors.Is(err, push.ErrGone) || hits.Load() != 1 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestSendNotFoundIsGone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := newSender(t).Send(context.Background(), testSub(t, srv.URL), testMsg); !errors.Is(err, push.ErrGone) {
		t.Fatalf("err=%v, want ErrGone", err)
	}
}

// A 5xx is temporary: Send retries 3 times (waits 1s and 2s between attempts).
func TestSendRetriesServerErrorThenGivesUp(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := newSender(t).Send(context.Background(), testSub(t, srv.URL), testMsg)
	if !errors.Is(err, push.ErrTemporary) || hits.Load() != 3 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
}

func TestSendNetworkErrorIsTemporary(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	endpoint := srv.URL
	srv.Close() // nothing listens here any more: connection refused

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := newSender(t).Send(ctx, testSub(t, endpoint), testMsg)
	if err == nil {
		t.Fatal("want error for unreachable push service")
	}
}

func TestSendCanceledContextReturnsContextError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := newSender(t).Send(ctx, testSub(t, srv.URL), testMsg)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context.Canceled", err)
	}
}
