package billing_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qms/payment-service/internal/billing"
)

// stripeStub menjalankan httptest server yang berpura-pura jadi API Stripe. Handler dipasang per test.
func stripeStub(t *testing.T, h http.HandlerFunc) string {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

func checkoutOK(w http.ResponseWriter, r *http.Request, seen *map[string]string) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/checkout/sessions" {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m := map[string]string{}
	for k := range r.PostForm {
		m[k] = r.PostForm.Get(k)
	}
	*seen = m
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "cs_test_1", "object": "checkout.session",
		"url": "https://checkout.stripe.test/c/cs_test_1", "expires_at": 1893456000,
	})
}

func TestStripeCreateCheckoutPriceID(t *testing.T) {
	var form map[string]string
	base := stripeStub(t, func(w http.ResponseWriter, r *http.Request) { checkoutOK(w, r, &form) })
	p := billing.NewStripeProvider("sk_test_x", base, secret, "price_123", 50000, "idr")

	co, err := p.CreateCheckout(context.Background(), billing.CheckoutInput{
		UserID: 42, CustomerEmail: "owner@test.com",
		SuccessURL: "https://qms.example.com/ok", CancelURL: "https://qms.example.com/no",
	})
	if err != nil {
		t.Fatal(err)
	}
	if co.URL != "https://checkout.stripe.test/c/cs_test_1" || co.ExpiresAt.Unix() != 1893456000 || co.ExpiresAt.Location().String() != "UTC" {
		t.Fatalf("checkout = %+v", co)
	}
	want := map[string]string{
		"mode": "payment", "client_reference_id": "42", "customer_email": "owner@test.com",
		"success_url": "https://qms.example.com/ok", "cancel_url": "https://qms.example.com/no",
		"line_items[0][price]": "price_123", "line_items[0][quantity]": "1",
		"metadata[user_id]": "42", "metadata[purpose]": "qms_subscription_1_month",
	}
	for k, v := range want {
		if form[k] != v {
			t.Errorf("param %s = %q, want %q", k, form[k], v)
		}
	}
	if _, ok := form["line_items[0][price_data][unit_amount]"]; ok {
		t.Error("price_data must not be sent when a price ID is configured")
	}
}

func TestStripeCreateCheckoutInlinePrice(t *testing.T) {
	var form map[string]string
	base := stripeStub(t, func(w http.ResponseWriter, r *http.Request) { checkoutOK(w, r, &form) })
	p := billing.NewStripeProvider("sk_test_x", base, secret, "", 50000, "idr")

	if _, err := p.CreateCheckout(context.Background(), billing.CheckoutInput{UserID: 3}); err != nil {
		t.Fatal(err)
	}
	// 50000 IDR = 5000000 satuan terkecil (sen).
	if form["line_items[0][price_data][unit_amount]"] != "5000000" || form["line_items[0][price_data][currency]"] != "idr" {
		t.Fatalf("inline price = %v", form)
	}
	if form["line_items[0][price_data][product_data][name]"] == "" {
		t.Fatal("inline price needs a product name")
	}
	if _, ok := form["line_items[0][price]"]; ok {
		t.Error("price ID must not be sent when none is configured")
	}
}

func TestStripeCreateCheckoutInlineZeroDecimal(t *testing.T) {
	var form map[string]string
	base := stripeStub(t, func(w http.ResponseWriter, r *http.Request) { checkoutOK(w, r, &form) })
	p := billing.NewStripeProvider("sk_test_x", base, secret, "", 1500, "jpy")
	if _, err := p.CreateCheckout(context.Background(), billing.CheckoutInput{UserID: 3}); err != nil {
		t.Fatal(err)
	}
	if form["line_items[0][price_data][unit_amount]"] != "1500" {
		t.Fatalf("unit_amount = %q, want 1500 for JPY", form["line_items[0][price_data][unit_amount]"])
	}
}

func TestStripeCreateCheckoutAPIError(t *testing.T) {
	base := stripeStub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"No such price"}}`)
	})
	p := billing.NewStripeProvider("sk_test_x", base, secret, "price_bad", 50000, "idr")
	if _, err := p.CreateCheckout(context.Background(), billing.CheckoutInput{UserID: 1}); err == nil {
		t.Fatal("expected error from Stripe 400")
	}

	// Lewat Service: galat Stripe dibungkus dengan konteks dan tidak dianggap ErrDisabled.
	st := &fakeStore{}
	svc := billing.New(st, p, "https://qms.example.com", slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := svc.StartCheckout(context.Background(), 1)
	if err == nil || !strings.Contains(err.Error(), "create checkout") || errors.Is(err, billing.ErrDisabled) {
		t.Fatalf("StartCheckout err = %v", err)
	}
}
