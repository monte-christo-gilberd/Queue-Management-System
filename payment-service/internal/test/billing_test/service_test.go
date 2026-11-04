package billing_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"qms/payment-service/internal/billing"
)

type fakeStore struct {
	ownerErr  error
	credits   []billing.Credit
	creditErr error
	extended  bool
	recorded  []string
	recordErr error
	pingErr   error
	expiry    time.Time
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }
func (f *fakeStore) OwnerEmail(context.Context, int64) (string, error) {
	return "owner@test.com", f.ownerErr
}
func (f *fakeStore) Credit(_ context.Context, c billing.Credit) (bool, time.Time, error) {
	f.credits = append(f.credits, c)
	return f.extended, f.expiry, f.creditErr
}
func (f *fakeStore) RecordEvent(_ context.Context, id, _ string) error {
	f.recorded = append(f.recorded, id)
	return f.recordErr
}

type fakeProvider struct {
	event      billing.Event
	parseErr   error
	checkout   billing.Checkout
	createErr  error
	lastCreate billing.CheckoutInput
}

func (f *fakeProvider) CreateCheckout(_ context.Context, in billing.CheckoutInput) (billing.Checkout, error) {
	f.lastCreate = in
	return f.checkout, f.createErr
}
func (f *fakeProvider) ParseEvent([]byte, string) (billing.Event, error) { return f.event, f.parseErr }

func newSvc(st *fakeStore, p *fakeProvider) *billing.Service {
	var prov billing.Provider
	if p != nil {
		prov = p
	}
	return billing.New(st, prov, "https://qms.example.com/", slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func paidEvent() billing.Event {
	return billing.Event{ID: "evt_1", Type: "checkout.session.completed", Session: billing.Session{
		ID: "cs_1", OwnerID: 7, PaymentStatus: "paid", AmountTotal: 5000000, Currency: "idr",
	}}
}

func TestPaidSessionExtendsOnce(t *testing.T) {
	st := &fakeStore{extended: true}
	svc := newSvc(st, &fakeProvider{event: paidEvent()})
	if err := svc.HandleWebhook(context.Background(), []byte("{}"), "sig"); err != nil {
		t.Fatal(err)
	}
	if len(st.credits) != 1 {
		t.Fatalf("credits = %d, want 1", len(st.credits))
	}
	c := st.credits[0]
	if c.OwnerID != 7 || c.SessionID != "cs_1" || c.EventID != "evt_1" || c.Amount != "50000.00" || c.Currency != "IDR" {
		t.Fatalf("credit = %+v", c)
	}
}

func TestDuplicateEventIsNotAnError(t *testing.T) {
	st := &fakeStore{extended: false} // store melaporkan sudah pernah diproses
	svc := newSvc(st, &fakeProvider{event: paidEvent()})
	if err := svc.HandleWebhook(context.Background(), nil, "sig"); err != nil {
		t.Fatalf("duplicate must be acknowledged, got %v", err)
	}
}

func TestUnpaidCompletedOnlyRecorded(t *testing.T) {
	ev := paidEvent()
	ev.Session.PaymentStatus = "unpaid"
	st := &fakeStore{}
	if err := newSvc(st, &fakeProvider{event: ev}).HandleWebhook(context.Background(), nil, "sig"); err != nil {
		t.Fatal(err)
	}
	if len(st.credits) != 0 || len(st.recorded) != 1 {
		t.Fatalf("credits=%d recorded=%d, want 0 and 1", len(st.credits), len(st.recorded))
	}
}

func TestOtherEventsOnlyRecorded(t *testing.T) {
	for _, typ := range []string{"checkout.session.expired", "checkout.session.async_payment_failed", "charge.refunded"} {
		st := &fakeStore{}
		ev := billing.Event{ID: "evt_x", Type: typ}
		if err := newSvc(st, &fakeProvider{event: ev}).HandleWebhook(context.Background(), nil, "sig"); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if len(st.credits) != 0 || len(st.recorded) != 1 {
			t.Fatalf("%s: credits=%d recorded=%d", typ, len(st.credits), len(st.recorded))
		}
	}
}

func TestAsyncPaymentSucceededExtends(t *testing.T) {
	ev := paidEvent()
	ev.Type = "checkout.session.async_payment_succeeded"
	st := &fakeStore{extended: true}
	if err := newSvc(st, &fakeProvider{event: ev}).HandleWebhook(context.Background(), nil, "sig"); err != nil {
		t.Fatal(err)
	}
	if len(st.credits) != 1 {
		t.Fatalf("credits = %d", len(st.credits))
	}
}

func TestBadSignature(t *testing.T) {
	svc := newSvc(&fakeStore{}, &fakeProvider{parseErr: errors.New("nope")})
	if err := svc.HandleWebhook(context.Background(), nil, "bad"); !errors.Is(err, billing.ErrBadSignature) {
		t.Fatalf("err = %v, want ErrBadSignature", err)
	}
}

func TestStoreFailureIsRetried(t *testing.T) {
	st := &fakeStore{creditErr: errors.New("db down")}
	err := newSvc(st, &fakeProvider{event: paidEvent()}).HandleWebhook(context.Background(), nil, "sig")
	if err == nil {
		t.Fatal("store failure must surface so Stripe retries")
	}
}

func TestUnknownOwnerNotRetried(t *testing.T) {
	st := &fakeStore{creditErr: billing.ErrNotOwner}
	if err := newSvc(st, &fakeProvider{event: paidEvent()}).HandleWebhook(context.Background(), nil, "sig"); err != nil {
		t.Fatalf("unknown owner needs a human, retrying is pointless; got %v", err)
	}
}

func TestPaidWithoutOwnerReferenceIgnored(t *testing.T) {
	ev := paidEvent()
	ev.Session.OwnerID = 0
	st := &fakeStore{}
	if err := newSvc(st, &fakeProvider{event: ev}).HandleWebhook(context.Background(), nil, "sig"); err != nil {
		t.Fatal(err)
	}
	if len(st.credits) != 0 {
		t.Fatal("must not credit without an owner")
	}
}

func TestDisabledWithoutProvider(t *testing.T) {
	svc := newSvc(&fakeStore{}, nil)
	if svc.Enabled() {
		t.Fatal("Enabled must be false without provider")
	}
	if _, err := svc.StartCheckout(context.Background(), 1); !errors.Is(err, billing.ErrDisabled) {
		t.Fatalf("StartCheckout err = %v", err)
	}
	if err := svc.HandleWebhook(context.Background(), nil, ""); !errors.Is(err, billing.ErrDisabled) {
		t.Fatalf("HandleWebhook err = %v", err)
	}
}

func TestStartCheckout(t *testing.T) {
	p := &fakeProvider{checkout: billing.Checkout{URL: "https://checkout.stripe.com/x"}}
	co, err := newSvc(&fakeStore{}, p).StartCheckout(context.Background(), 7)
	if err != nil || co.URL != "https://checkout.stripe.com/x" {
		t.Fatalf("co=%+v err=%v", co, err)
	}
	if p.lastCreate.OwnerID != 7 || p.lastCreate.CustomerEmail != "owner@test.com" ||
		p.lastCreate.SuccessURL != "https://qms.example.com/subscription?payment=success" ||
		p.lastCreate.CancelURL != "https://qms.example.com/subscription?payment=cancel" {
		t.Fatalf("input = %+v", p.lastCreate)
	}
}

func TestStartCheckoutNotOwner(t *testing.T) {
	_, err := newSvc(&fakeStore{ownerErr: billing.ErrNotOwner}, &fakeProvider{}).StartCheckout(context.Background(), 9)
	if !errors.Is(err, billing.ErrNotOwner) {
		t.Fatalf("err = %v", err)
	}
}

func TestMinorFactor(t *testing.T) {
	for cur, want := range map[string]int64{"idr": 100, "USD": 100, "jpy": 1, "VND": 1} {
		if got := billing.MinorFactor(cur); got != want {
			t.Errorf("MinorFactor(%s) = %d, want %d", cur, got, want)
		}
	}
}
