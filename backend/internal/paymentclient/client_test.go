package paymentclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qms/internal/apperror"
)

const testToken = "svc_token_0123456789abcdef0123456789"

func newClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New(srv.URL, testToken, 5*time.Second)
}

var sampleReq = CheckoutRequest{OwnerID: 7, Amount: 15000, Method: "qris", ReferenceID: "SUB-7-1-abc", PlanType: "monthly"}

func TestCheckout_SendsServiceTokenAndParses(t *testing.T) {
	var gotToken, gotPath string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Service-Token")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Error("body kosong")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"reference_id":"SUB-7-1-abc","status":"pending","pay_url":"https://borderpay.id/pay/x","expires_at":"2026-11-13T04:00:00Z"}`))
	})
	out, err := c.Checkout(context.Background(), sampleReq)
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if gotToken != testToken || gotPath != "/internal/v1/payments" {
		t.Fatalf("token=%q path=%q", gotToken, gotPath)
	}
	if out.PayURL != "https://borderpay.id/pay/x" {
		t.Fatalf("pay_url = %q", out.PayURL)
	}
}

func TestCheckout_StatusMapping(t *testing.T) {
	cases := map[int]error{
		http.StatusBadRequest:         apperror.ErrInvalidPaymentMethod,
		http.StatusConflict:           apperror.ErrPaymentRejected,
		http.StatusUnauthorized:       apperror.ErrPaymentRejected,
		http.StatusBadGateway:         apperror.ErrPaymentUnavailable,
		http.StatusServiceUnavailable: apperror.ErrPaymentUnavailable,
		http.StatusTooManyRequests:    apperror.ErrPaymentUnavailable,
	}
	for status, want := range cases {
		c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"rahasia internal"}`))
		})
		if _, err := c.Checkout(context.Background(), sampleReq); !errors.Is(err, want) {
			t.Errorf("status %d: err = %v, want %v", status, err, want)
		}
	}
}

func TestCheckout_UnreachableIsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := srv.URL
	srv.Close()
	c := New(url, testToken, time.Second)
	if _, err := c.Checkout(context.Background(), sampleReq); !errors.Is(err, apperror.ErrPaymentUnavailable) {
		t.Fatalf("err = %v, want ErrPaymentUnavailable", err)
	}
}

func TestCheckout_EmptyPayURLIsUnavailable(t *testing.T) {
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"reference_id":"SUB-7-1-abc"}`))
	})
	if _, err := c.Checkout(context.Background(), sampleReq); !errors.Is(err, apperror.ErrPaymentUnavailable) {
		t.Fatalf("err = %v, want ErrPaymentUnavailable", err)
	}
}
