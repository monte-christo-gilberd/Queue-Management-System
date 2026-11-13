package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"qms/internal/apperror"
	"qms/internal/paymentclient"
)

// PaymentCheckout: kontrak ke payment-service (dipenuhi *paymentclient.Client).
type PaymentCheckout interface {
	Checkout(ctx context.Context, req paymentclient.CheckoutRequest) (paymentclient.CheckoutResponse, error)
}

// planMonthly: paket yang dijual lewat checkout. Sama dengan order.PlanMonthly di payment-service.
const planMonthly = "monthly"

// SubscriptionCheckout membuat pesanan pembayaran langganan lewat payment-service. Harga dan ID pesanan
// ditentukan di backend; payment-service hanya meneruskan ke BorderPay dan mencatat pesanan.
type SubscriptionCheckout struct {
	client       PaymentCheckout
	priceIDR     int64
	verification VerificationGuard
	now          func() time.Time
}

// CheckoutInput: pilihan metode pembayaran dari frontend.
type CheckoutInput struct {
	// Method: hanya qris.
	Method string
	// ReturnURL: halaman tujuan setelah membayar (opsional).
	ReturnURL string
}

// CheckoutResult: data pesanan yang dikembalikan ke frontend.
type CheckoutResult struct {
	ReferenceID string
	PayURL      string
	ExpiresAt   string
	Amount      int64
}

// NewSubscriptionCheckout membuat layanan checkout dengan harga paket bulanan dalam rupiah.
func NewSubscriptionCheckout(client PaymentCheckout, priceIDR int64) *SubscriptionCheckout {
	return &SubscriptionCheckout{client: client, priceIDR: priceIDR, now: time.Now}
}

// WithVerificationGuard mewajibkan email terverifikasi sebelum checkout (bila verifikasi email dinyalakan).
func (s *SubscriptionCheckout) WithVerificationGuard(guard VerificationGuard) *SubscriptionCheckout {
	s.verification = guard
	return s
}

// Checkout membuat pesanan pending untuk ownerID dan mengembalikan pay_url untuk dibuka customer.
func (s *SubscriptionCheckout) Checkout(ctx context.Context, ownerID int64, in CheckoutInput) (CheckoutResult, error) {
	if s.verification != nil {
		if err := s.verification.RequireVerified(ctx, ownerID); err != nil {
			return CheckoutResult{}, err
		}
	}

	// Hanya QRIS yang didukung; metode lain ditolak sebelum payment-service dipanggil.
	method := strings.ToLower(strings.TrimSpace(in.Method))
	if method != "qris" {
		return CheckoutResult{}, apperror.ErrInvalidPaymentMethod
	}

	refID, err := s.newReferenceID(ownerID)
	if err != nil {
		return CheckoutResult{}, err
	}

	out, err := s.client.Checkout(ctx, paymentclient.CheckoutRequest{
		OwnerID:     ownerID,
		Amount:      s.priceIDR,
		Method:      method,
		ReturnURL:   strings.TrimSpace(in.ReturnURL),
		ReferenceID: refID,
		PlanType:    planMonthly,
	})
	if err != nil {
		return CheckoutResult{}, err
	}
	return CheckoutResult{
		ReferenceID: out.ReferenceID,
		PayURL:      out.PayURL,
		ExpiresAt:   out.ExpiresAt,
		Amount:      s.priceIDR,
	}, nil
}

// newReferenceID membuat ID pesanan unik: SUB-<owner>-<waktu>-<acak>. Acak dari crypto/rand.
func (s *SubscriptionCheckout) newReferenceID(ownerID int64) (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("subscription checkout: acak: %w", err)
	}
	return fmt.Sprintf("SUB-%d-%d-%s", ownerID, s.now().Unix(), hex.EncodeToString(buf)), nil
}
