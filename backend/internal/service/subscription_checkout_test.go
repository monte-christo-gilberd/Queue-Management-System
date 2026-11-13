package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"qms/internal/apperror"
	"qms/internal/paymentclient"
)

// fakePayment mencatat request checkout dan membalas sesuai err.
type fakePayment struct {
	got paymentclient.CheckoutRequest
	err error
}

func (f *fakePayment) Checkout(_ context.Context, req paymentclient.CheckoutRequest) (paymentclient.CheckoutResponse, error) {
	f.got = req
	if f.err != nil {
		return paymentclient.CheckoutResponse{}, f.err
	}
	return paymentclient.CheckoutResponse{ReferenceID: req.ReferenceID, PayURL: "https://borderpay.id/pay/" + req.ReferenceID, ExpiresAt: "2026-11-13T04:00:00Z"}, nil
}

// fakeGuard meniru verifikasi email: err dikembalikan bila email belum terverifikasi.
type fakeGuard struct{ err error }

func (g fakeGuard) RequireVerified(context.Context, int64) error { return g.err }

func TestSubscriptionCheckout_UsesServerPriceAndReference(t *testing.T) {
	pay := &fakePayment{}
	svc := NewSubscriptionCheckout(pay, 15000)

	res, err := svc.Checkout(context.Background(), 7, CheckoutInput{Method: " QRIS ", ReturnURL: "https://app/ok"})
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	if pay.got.Amount != 15000 || pay.got.OwnerID != 7 || pay.got.PlanType != "monthly" || pay.got.Method != "qris" {
		t.Fatalf("request ke payment-service salah: %+v", pay.got)
	}
	if !strings.HasPrefix(pay.got.ReferenceID, "SUB-7-") {
		t.Fatalf("reference_id = %q", pay.got.ReferenceID)
	}
	if res.PayURL == "" || res.Amount != 15000 || res.ReferenceID != pay.got.ReferenceID {
		t.Fatalf("hasil salah: %+v", res)
	}
}

func TestSubscriptionCheckout_ReferenceIDsAreUnique(t *testing.T) {
	pay := &fakePayment{}
	svc := NewSubscriptionCheckout(pay, 15000)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		if _, err := svc.Checkout(context.Background(), 7, CheckoutInput{Method: "qris"}); err != nil {
			t.Fatal(err)
		}
		if seen[pay.got.ReferenceID] {
			t.Fatalf("reference_id duplikat: %s", pay.got.ReferenceID)
		}
		seen[pay.got.ReferenceID] = true
	}
}

func TestSubscriptionCheckout_ValidatesMethod(t *testing.T) {
	cases := []CheckoutInput{
		{Method: "cash"},
		{Method: "va"},
		{Method: "ewallet", BankCode: " "},
		{Method: ""},
	}
	for _, in := range cases {
		pay := &fakePayment{}
		svc := NewSubscriptionCheckout(pay, 15000)
		if _, err := svc.Checkout(context.Background(), 7, in); !errors.Is(err, apperror.ErrInvalidPaymentMethod) {
			t.Errorf("%+v: err = %v, want ErrInvalidPaymentMethod", in, err)
		}
		if pay.got.Amount != 0 {
			t.Errorf("%+v: payment-service tidak boleh dipanggil", in)
		}
	}
}

func TestSubscriptionCheckout_VANeedsBankCode(t *testing.T) {
	pay := &fakePayment{}
	svc := NewSubscriptionCheckout(pay, 15000)
	if _, err := svc.Checkout(context.Background(), 7, CheckoutInput{Method: "va", BankCode: "bca"}); err != nil {
		t.Fatalf("va dengan bank: %v", err)
	}
	if pay.got.BankCode != "BCA" {
		t.Fatalf("bank_code = %q, want BCA", pay.got.BankCode)
	}
}

func TestSubscriptionCheckout_RequiresVerifiedEmail(t *testing.T) {
	pay := &fakePayment{}
	svc := NewSubscriptionCheckout(pay, 15000).WithVerificationGuard(fakeGuard{err: apperror.ErrEmailNotVerified})
	if _, err := svc.Checkout(context.Background(), 7, CheckoutInput{Method: "qris"}); !errors.Is(err, apperror.ErrEmailNotVerified) {
		t.Fatalf("err = %v, want ErrEmailNotVerified", err)
	}
	if pay.got.Amount != 0 {
		t.Fatal("payment-service tidak boleh dipanggil sebelum email terverifikasi")
	}
}

func TestSubscriptionCheckout_PropagatesPaymentErrors(t *testing.T) {
	svc := NewSubscriptionCheckout(&fakePayment{err: apperror.ErrPaymentUnavailable}, 15000)
	if _, err := svc.Checkout(context.Background(), 7, CheckoutInput{Method: "qris"}); !errors.Is(err, apperror.ErrPaymentUnavailable) {
		t.Fatalf("err = %v, want ErrPaymentUnavailable", err)
	}
}
