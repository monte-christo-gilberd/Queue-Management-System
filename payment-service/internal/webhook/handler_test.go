package webhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qms/payment-service/internal/order"
)

const testToken = "bpt_test_token_123"

// fakeStore mencatat pemanggilan MarkPaid dan meniru idempotency dengan set referenceID.
type fakeStore struct {
	calls   int
	applied map[string]bool
	err     error
}

func (f *fakeStore) MarkPaid(_ context.Context, ref string, _ int64) (bool, error) {
	f.calls++
	if f.err != nil {
		return false, f.err
	}
	if f.applied == nil {
		f.applied = map[string]bool{}
	}
	if f.applied[ref] {
		return false, nil
	}
	f.applied[ref] = true
	return true, nil
}

func post(t *testing.T, h http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/borderpay", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("x-borderpay-token", token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const paidBody = `{"event":"payment.paid","mode":"test","data":{"reference_id":"ORD-123","status":"paid","amount":150000}}`

func TestWebhook_RejectsMissingOrWrongToken(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(testToken, store)

	// Tanpa header token: ditolak dan store tidak disentuh.
	if rec := post(t, h, "", paidBody); rec.Code != http.StatusUnauthorized {
		t.Fatalf("tanpa token: status = %d, want 401", rec.Code)
	}
	// Token salah: ditolak.
	if rec := post(t, h, "bpt_salah", paidBody); rec.Code != http.StatusUnauthorized {
		t.Fatalf("token salah: status = %d, want 401", rec.Code)
	}
	if store.calls != 0 {
		t.Fatalf("store dipanggil %d kali walau token salah", store.calls)
	}
}

func TestWebhook_FailsClosedWithoutConfiguredToken(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler("", store)

	// Token kosong di konfigurasi: semua request ditolak, termasuk yang membawa header kosong.
	if rec := post(t, h, "", paidBody); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if store.calls != 0 {
		t.Fatal("store tidak boleh dipanggil tanpa token terkonfigurasi")
	}
}

func TestWebhook_PaidAppliesOnceAndAcknowledges(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(testToken, store)

	rec := post(t, h, testToken, paidBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("paid pertama: status = %d, want 200", rec.Code)
	}
	if store.calls != 1 || !store.applied["ORD-123"] {
		t.Fatalf("paid pertama harus diterapkan sekali, calls=%d", store.calls)
	}

	// Pengiriman ulang (duplikat) tetap dibalas 200 tetapi tidak diterapkan lagi.
	rec = post(t, h, testToken, paidBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplikat: status = %d, want 200", rec.Code)
	}
	if store.calls != 2 {
		t.Fatalf("store harus dipanggil untuk tiap kiriman, calls=%d", store.calls)
	}
}

func TestWebhook_NonPaidStatusIsIgnored(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(testToken, store)

	for _, status := range []string{"expired", "failed", "pending"} {
		body := `{"event":"payment.expired","data":{"reference_id":"ORD-9","status":"` + status + `","amount":1}}`
		if rec := post(t, h, testToken, body); rec.Code != http.StatusOK {
			t.Fatalf("status %s: code = %d, want 200", status, rec.Code)
		}
	}
	if store.calls != 0 {
		t.Fatalf("status selain paid tidak boleh mengubah data, calls=%d", store.calls)
	}
}

func TestWebhook_BadRequests(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(testToken, store)

	if rec := post(t, h, testToken, `{bukan json`); rec.Code != http.StatusBadRequest {
		t.Fatalf("json rusak: status = %d, want 400", rec.Code)
	}
	noRef := `{"event":"payment.paid","data":{"status":"paid","amount":1}}`
	if rec := post(t, h, testToken, noRef); rec.Code != http.StatusBadRequest {
		t.Fatalf("tanpa reference_id: status = %d, want 400", rec.Code)
	}
	huge := strings.Repeat("a", maxBodyBytes+10)
	if rec := post(t, h, testToken, huge); rec.Code != http.StatusBadRequest {
		t.Fatalf("body terlalu besar: status = %d, want 400", rec.Code)
	}
}

func TestWebhook_StoreErrorAsksForRetry(t *testing.T) {
	store := &fakeStore{err: errors.New("db mati")}
	h := NewHandler(testToken, store)

	// Galat penyimpanan harus 500 agar BorderPay mengirim ulang, bukan 200 yang menelan pembayaran.
	if rec := post(t, h, testToken, paidBody); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestWebhook_UnknownReferenceIsAcknowledged(t *testing.T) {
	// Pesanan tidak dikenal tidak bisa diterapkan; dibalas 200 supaya BorderPay tidak mengulang terus.
	h := NewHandler(testToken, &fakeStore{err: order.ErrUnknown})
	if rec := post(t, h, testToken, paidBody); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestWebhook_AmountMismatchIsConflict(t *testing.T) {
	h := NewHandler(testToken, &fakeStore{err: order.ErrAmountMismatch})
	if rec := post(t, h, testToken, paidBody); rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}
