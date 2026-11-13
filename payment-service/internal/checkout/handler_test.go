package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qms/payment-service/internal/borderpay"
)

const testServiceToken = "svc_token_0123456789abcdef0123456789"

type fakeCreator struct {
	got borderpay.CreateRequest
	out borderpay.CreateResponse
	err error
}

func (f *fakeCreator) CreatePayment(_ context.Context, req borderpay.CreateRequest) (borderpay.CreateResponse, error) {
	f.got = req
	return f.out, f.err
}

func do(t *testing.T, h http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/payments", strings.NewReader(body))
	if token != "" {
		req.Header.Set("X-Service-Token", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCheckout_RequiresServiceToken(t *testing.T) {
	creator := &fakeCreator{}
	h := NewHandler(testServiceToken, creator)
	if rec := do(t, h, "", `{"amount":1,"method":"qris"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token: status = %d, want 401", rec.Code)
	}
	if rec := do(t, h, "salah", `{"amount":1,"method":"qris"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token salah: status = %d, want 401", rec.Code)
	}
}

func TestCheckout_FailsClosedWithoutConfiguredToken(t *testing.T) {
	h := NewHandler("", &fakeCreator{})
	if rec := do(t, h, "", `{"amount":1,"method":"qris"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCheckout_ReturnsPayURL(t *testing.T) {
	creator := &fakeCreator{out: borderpay.CreateResponse{
		ReferenceID: "ORD-123", Status: "pending", PayURL: "https://borderpay.id/pay/ORD-123", ExpiresAt: "2026-07-13T11:00:00Z",
	}}
	h := NewHandler(testServiceToken, creator)

	rec := do(t, h, testServiceToken, `{"amount":150000,"method":"va","bank_code":" BCA ","reference_id":"ORD-123","return_url":"https://toko.com/ok"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if creator.got.BankCode != "BCA" || creator.got.ReferenceID != "ORD-123" || creator.got.Amount != 150000 {
		t.Fatalf("request ke BorderPay salah: %+v", creator.got)
	}
	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.PayURL != "https://borderpay.id/pay/ORD-123" || resp.ReferenceID != "ORD-123" {
		t.Fatalf("respons salah: %+v", resp)
	}
}

func TestCheckout_ErrorMapping(t *testing.T) {
	h := NewHandler(testServiceToken, &fakeCreator{err: borderpay.ErrInvalidRequest})
	if rec := do(t, h, testServiceToken, `{"amount":0,"method":"qris"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("validasi: status = %d, want 400", rec.Code)
	}

	h = NewHandler(testServiceToken, &fakeCreator{err: errors.New("detail internal")})
	rec := do(t, h, testServiceToken, `{"amount":1,"method":"qris"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("upstream: status = %d, want 502", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "detail internal") {
		t.Fatal("detail galat internal tidak boleh bocor ke pemanggil")
	}
}

func TestCheckout_BadJSON(t *testing.T) {
	h := NewHandler(testServiceToken, &fakeCreator{})
	if rec := do(t, h, testServiceToken, `{rusak`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
