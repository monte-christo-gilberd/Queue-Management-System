package borderpay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testKey = "bp_test_key_123"

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := New(srv.URL, testKey, 5*time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestCreatePayment_SendsAuthAndParsesPayURL(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody CreateRequest
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"reference_id":"ORD-123","status":"pending","amount":150000,"pay_url":"https://borderpay.id/pay/ORD-123","expires_at":"2026-07-13T11:00:00Z"}`))
	})

	out, err := c.CreatePayment(context.Background(), CreateRequest{Amount: 150000, Method: MethodQRIS, ReferenceID: "ORD-123"})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if gotAuth != "Bearer "+testKey {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPath != "/api/v1/payments" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotBody.Amount != 150000 || gotBody.Method != MethodQRIS || gotBody.ReferenceID != "ORD-123" {
		t.Fatalf("body terkirim salah: %+v", gotBody)
	}
	if out.PayURL != "https://borderpay.id/pay/ORD-123" || out.ReferenceID != "ORD-123" {
		t.Fatalf("respons salah: %+v", out)
	}
}

func TestCreatePayment_UpstreamErrorHidesBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"key rahasia salah"}`))
	})
	_, err := c.CreatePayment(context.Background(), CreateRequest{Amount: 1, Method: MethodQRIS})
	if !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}

func TestCreatePayment_RejectsEmptyPayURL(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"reference_id":"ORD-1","status":"pending"}`))
	})
	if _, err := c.CreatePayment(context.Background(), CreateRequest{Amount: 1, Method: MethodQRIS}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		req  CreateRequest
		ok   bool
	}{
		{"qris valid", CreateRequest{Amount: 1, Method: MethodQRIS}, true},
		{"nominal nol", CreateRequest{Amount: 0, Method: MethodQRIS}, false},
		{"va ditolak", CreateRequest{Amount: 1, Method: "va"}, false},
		{"ewallet ditolak", CreateRequest{Amount: 1, Method: "ewallet"}, false},
		{"metode tidak dikenal", CreateRequest{Amount: 1, Method: "cash"}, false},
	}
	for _, tc := range cases {
		err := tc.req.Validate()
		if tc.ok && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("%s: err = %v, want ErrInvalidRequest", tc.name, err)
		}
	}
}

func TestCreatePayment_InvalidRequestNotSent(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})
	if _, err := c.CreatePayment(context.Background(), CreateRequest{Amount: 0, Method: MethodQRIS}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v, want ErrInvalidRequest", err)
	}
	if called {
		t.Fatal("request tidak valid tidak boleh dikirim ke BorderPay")
	}
}

func TestNew_RequiresBaseURLAndKey(t *testing.T) {
	if _, err := New("", testKey, time.Second); err == nil {
		t.Fatal("base URL kosong harus ditolak")
	}
	if _, err := New("https://borderpay.id", "", time.Second); err == nil {
		t.Fatal("API key kosong harus ditolak")
	}
}
