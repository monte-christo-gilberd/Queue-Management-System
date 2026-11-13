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
	"qms/payment-service/internal/order"
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

// fakeRecorder mencatat pesanan dan pay_url; createErr dipakai untuk meniru reference_id ganda.
type fakeRecorder struct {
	pending   []order.Pending
	payURLs   map[string]string
	createErr error
	setErr    error
}

func (f *fakeRecorder) CreatePending(_ context.Context, o order.Pending) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.pending = append(f.pending, o)
	return nil
}

func (f *fakeRecorder) SetPayURL(_ context.Context, ref, payURL string) error {
	if f.setErr != nil {
		return f.setErr
	}
	if f.payURLs == nil {
		f.payURLs = map[string]string{}
	}
	f.payURLs[ref] = payURL
	return nil
}

const validBody = `{"owner_id":7,"amount":150000,"method":"qris","reference_id":"ORD-123"}`

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
	h := NewHandler(testServiceToken, creator, &fakeRecorder{})
	if rec := do(t, h, "", `{"amount":1,"method":"qris"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token: status = %d, want 401", rec.Code)
	}
	if rec := do(t, h, "salah", `{"amount":1,"method":"qris"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token salah: status = %d, want 401", rec.Code)
	}
}

func TestCheckout_FailsClosedWithoutConfiguredToken(t *testing.T) {
	h := NewHandler("", &fakeCreator{}, &fakeRecorder{})
	if rec := do(t, h, "", `{"amount":1,"method":"qris"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCheckout_ReturnsPayURL(t *testing.T) {
	creator := &fakeCreator{out: borderpay.CreateResponse{
		ReferenceID: "ORD-123", Status: "pending", PayURL: "https://borderpay.id/pay/ORD-123", ExpiresAt: "2026-07-13T11:00:00Z",
	}}
	rec0 := &fakeRecorder{}
	h := NewHandler(testServiceToken, creator, rec0)

	rec := do(t, h, testServiceToken, `{"owner_id":7,"amount":150000,"method":"qris","reference_id":"ORD-123","return_url":"https://toko.com/ok"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if creator.got.ReferenceID != "ORD-123" || creator.got.Amount != 150000 || creator.got.Method != "qris" {
		t.Fatalf("request ke BorderPay salah: %+v", creator.got)
	}
	if len(rec0.pending) != 1 || rec0.pending[0].OwnerID != 7 || rec0.pending[0].PlanType != order.PlanMonthly {
		t.Fatalf("pesanan pending tidak tercatat dengan benar: %+v", rec0.pending)
	}
	if rec0.payURLs["ORD-123"] != "https://borderpay.id/pay/ORD-123" {
		t.Fatalf("pay_url tidak tersimpan: %+v", rec0.payURLs)
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
	h := NewHandler(testServiceToken, &fakeCreator{err: borderpay.ErrInvalidRequest}, &fakeRecorder{})
	if rec := do(t, h, testServiceToken, `{"owner_id":7,"reference_id":"ORD-1","amount":0,"method":"qris"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("validasi: status = %d, want 400", rec.Code)
	}

	h = NewHandler(testServiceToken, &fakeCreator{err: errors.New("detail internal")}, &fakeRecorder{})
	rec := do(t, h, testServiceToken, `{"owner_id":7,"reference_id":"ORD-1","amount":1,"method":"qris"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("upstream: status = %d, want 502", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "detail internal") {
		t.Fatal("detail galat internal tidak boleh bocor ke pemanggil")
	}
}

func TestCheckout_BadJSON(t *testing.T) {
	h := NewHandler(testServiceToken, &fakeCreator{}, &fakeRecorder{})
	if rec := do(t, h, testServiceToken, `{rusak`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCheckout_RequiresReferenceAndOwner(t *testing.T) {
	h := NewHandler(testServiceToken, &fakeCreator{}, &fakeRecorder{})
	if rec := do(t, h, testServiceToken, `{"owner_id":7,"amount":1,"method":"qris"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("tanpa reference_id: status = %d, want 400", rec.Code)
	}
	if rec := do(t, h, testServiceToken, `{"amount":1,"method":"qris","reference_id":"ORD-1"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("tanpa owner_id: status = %d, want 400", rec.Code)
	}
}

func TestCheckout_DuplicateReferenceIsConflict(t *testing.T) {
	creator := &fakeCreator{}
	h := NewHandler(testServiceToken, creator, &fakeRecorder{createErr: order.ErrDuplicate})
	if rec := do(t, h, testServiceToken, validBody); rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	if creator.got.Amount != 0 {
		t.Fatal("BorderPay tidak boleh dipanggil untuk reference_id ganda")
	}
}

func TestCheckout_SetPayURLFailureIsServerError(t *testing.T) {
	h := NewHandler(testServiceToken, &fakeCreator{out: borderpay.CreateResponse{ReferenceID: "ORD-123", PayURL: "https://x"}},
		&fakeRecorder{setErr: errors.New("db")})
	if rec := do(t, h, testServiceToken, validBody); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
