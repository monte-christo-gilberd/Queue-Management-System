package billing_test

import (
	"testing"
	"time"

	"github.com/stripe/stripe-go/v85/webhook"

	"qms/payment-service/internal/billing"
)

const secret = "whsec_test_secret"

func signed(t *testing.T, payload string) (string, string) {
	t.Helper()
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: []byte(payload), Secret: secret, Timestamp: time.Now()})
	return string(sp.Payload), sp.Header
}

// api_version sengaja tidak sama dengan versi stripe-go: ParseEvent harus tetap menerimanya.
const completed = `{"id":"evt_1","object":"event","api_version":"2019-01-01","type":"checkout.session.completed",
"data":{"object":{"id":"cs_9","object":"checkout.session","client_reference_id":"42","payment_status":"paid","amount_total":5000000,"currency":"idr"}}}`

func TestParseEventValid(t *testing.T) {
	payload, sig := signed(t, completed)
	p := billing.NewStripeProvider("sk_test_x", "", secret, "", 50000, "idr")
	ev, err := p.ParseEvent([]byte(payload), sig)
	if err != nil {
		t.Fatal(err)
	}
	want := billing.Session{ID: "cs_9", UserID: 42, PaymentStatus: "paid", AmountTotal: 5000000, Currency: "idr"}
	if ev.ID != "evt_1" || ev.Type != "checkout.session.completed" || ev.Session != want {
		t.Fatalf("event = %+v", ev)
	}
}

func TestParseEventRejectsBadSignature(t *testing.T) {
	payload, _ := signed(t, completed)
	p := billing.NewStripeProvider("sk_test_x", "", secret, "", 50000, "idr")
	if _, err := p.ParseEvent([]byte(payload), "t=1,v1=deadbeef"); err == nil {
		t.Fatal("forged signature accepted")
	}
	if _, err := p.ParseEvent([]byte(payload), ""); err == nil {
		t.Fatal("missing signature accepted")
	}
}

func TestParseEventRejectsTamperedPayload(t *testing.T) {
	payload, sig := signed(t, completed)
	tampered := []byte(string(payload[:len(payload)-3]) + `"}}`)
	p := billing.NewStripeProvider("sk_test_x", "", secret, "", 50000, "idr")
	if _, err := p.ParseEvent(tampered, sig); err == nil {
		t.Fatal("tampered payload accepted")
	}
}

func TestParseEventWrongSecret(t *testing.T) {
	payload, sig := signed(t, completed)
	p := billing.NewStripeProvider("sk_test_x", "", "whsec_other", "", 50000, "idr")
	if _, err := p.ParseEvent([]byte(payload), sig); err == nil {
		t.Fatal("signature from another secret accepted")
	}
}
