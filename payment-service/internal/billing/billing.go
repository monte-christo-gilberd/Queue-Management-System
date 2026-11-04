// Package billing: satu-satunya kemampuan payment-service adalah memperpanjang langganan owner SATU BULAN
// setelah Stripe mengonfirmasi pembayaran. Tidak ada plan lain, tidak ada penagihan berulang.
package billing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrNotOwner: user ada tetapi bukan owner (atau tidak ada), jadi tidak punya langganan untuk diperpanjang.
	ErrNotOwner = errors.New("user is not an owner")
	// ErrDisabled: kunci Stripe belum dikonfigurasi; pembayaran dimatikan.
	ErrDisabled = errors.New("payments are not configured")
	// ErrBadSignature: tanda tangan webhook tidak sah.
	ErrBadSignature = errors.New("invalid webhook signature")
)

// CheckoutInput: sesi Checkout yang akan dibuka untuk satu owner.
type CheckoutInput struct {
	OwnerID       int64
	CustomerEmail string
	SuccessURL    string
	CancelURL     string
}

type Checkout struct {
	URL       string
	ExpiresAt time.Time
}

// Session: bagian sesi Checkout yang kita butuhkan dari webhook.
type Session struct {
	ID            string
	OwnerID       int64 // dari client_reference_id; 0 bila tidak ada/tidak valid
	PaymentStatus string
	AmountTotal   int64 // satuan terkecil mata uang
	Currency      string
}

type Event struct {
	ID      string
	Type    string
	Session Session
}

// Provider: sisi Stripe.
type Provider interface {
	CreateCheckout(ctx context.Context, in CheckoutInput) (Checkout, error)
	ParseEvent(payload []byte, sigHeader string) (Event, error)
}

// Credit: satu pembayaran yang harus mengubah langganan.
type Credit struct {
	EventID   string
	EventType string
	SessionID string
	OwnerID   int64
	Amount    string // satuan utama, desimal teks (mis. "50000.00")
	Currency  string // huruf besar
}

// Store: sisi database.
type Store interface {
	Ping(ctx context.Context) error
	// OwnerEmail mengembalikan ErrNotOwner bila user bukan owner.
	OwnerEmail(ctx context.Context, ownerID int64) (string, error)
	// Credit memperpanjang langganan satu bulan dan mencatat pembayaran, atomik dan idempoten per event dan
	// per sesi. extended=false bila event/sesi sudah pernah diproses.
	Credit(ctx context.Context, c Credit) (extended bool, expiresAt time.Time, err error)
	// RecordEvent mencatat event yang tidak mengubah langganan (supaya kiriman ulang diabaikan).
	RecordEvent(ctx context.Context, eventID, eventType string) error
}

type Service struct {
	store      Store
	provider   Provider // nil = pembayaran dimatikan
	log        *slog.Logger
	publicBase string
}

func New(store Store, provider Provider, publicOrigin string, log *slog.Logger) *Service {
	return &Service{store: store, provider: provider, publicBase: strings.TrimRight(publicOrigin, "/"), log: log}
}

func (s *Service) Enabled() bool { return s.provider != nil }

func (s *Service) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.store.Ping(ctx)
}

// StartCheckout membuka sesi pembayaran untuk perpanjangan satu bulan.
func (s *Service) StartCheckout(ctx context.Context, ownerID int64) (Checkout, error) {
	if s.provider == nil {
		return Checkout{}, ErrDisabled
	}
	email, err := s.store.OwnerEmail(ctx, ownerID)
	if err != nil {
		return Checkout{}, err
	}
	co, err := s.provider.CreateCheckout(ctx, CheckoutInput{
		OwnerID:       ownerID,
		CustomerEmail: email,
		SuccessURL:    s.publicBase + "/subscription?payment=success",
		CancelURL:     s.publicBase + "/subscription?payment=cancel",
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("create checkout: %w", err)
	}
	s.log.Info("checkout created", "owner_id", ownerID)
	return co, nil
}

// HandleWebhook memverifikasi tanda tangan lalu memperpanjang langganan bila pembayaran berhasil. Sumber
// kebenaran adalah webhook, bukan redirect browser. Mengembalikan error (Stripe akan mengirim ulang) hanya
// bila penyimpanan gagal; event yang tidak relevan atau owner yang hilang tidak dicoba ulang.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	if s.provider == nil {
		return ErrDisabled
	}
	ev, err := s.provider.ParseEvent(payload, sigHeader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadSignature, err)
	}

	switch ev.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		// completed juga terkirim untuk metode bayar tertunda (status unpaid); perpanjang hanya saat lunas.
		if ev.Session.PaymentStatus != "paid" {
			return s.store.RecordEvent(ctx, ev.ID, ev.Type)
		}
		if ev.Session.OwnerID <= 0 {
			s.log.Error("paid session without owner reference", "event_id", ev.ID, "session_id", ev.Session.ID)
			return nil
		}
		amount, err := majorUnits(ev.Session.AmountTotal, ev.Session.Currency)
		if err != nil {
			s.log.Error("bad amount on paid session", "event_id", ev.ID, "session_id", ev.Session.ID, "err", err)
			return nil
		}
		extended, expires, err := s.store.Credit(ctx, Credit{
			EventID:   ev.ID,
			EventType: ev.Type,
			SessionID: ev.Session.ID,
			OwnerID:   ev.Session.OwnerID,
			Amount:    amount,
			Currency:  strings.ToUpper(ev.Session.Currency),
		})
		switch {
		case errors.Is(err, ErrNotOwner):
			// Uang masuk tetapi owner tidak ada: butuh tindakan manusia, mengulang tidak menolong.
			s.log.Error("PAID SESSION FOR UNKNOWN OWNER, refund manually", "event_id", ev.ID,
				"session_id", ev.Session.ID, "owner_id", ev.Session.OwnerID)
			return nil
		case err != nil:
			return err
		case extended:
			s.log.Info("subscription extended", "owner_id", ev.Session.OwnerID,
				"session_id", ev.Session.ID, "expires_at", expires.UTC().Format(time.RFC3339))
		default:
			s.log.Info("duplicate payment event ignored", "event_id", ev.ID, "session_id", ev.Session.ID)
		}
		return nil
	default:
		// Event lain (expired, async_payment_failed, ...) tidak mengubah langganan; cukup dicatat.
		return s.store.RecordEvent(ctx, ev.ID, ev.Type)
	}
}

// zeroDecimal: mata uang yang di Stripe tidak punya satuan pecahan.
var zeroDecimal = map[string]bool{
	"bif": true, "clp": true, "djf": true, "gnf": true, "jpy": true, "kmf": true, "krw": true, "mga": true,
	"pyg": true, "rwf": true, "ugx": true, "vnd": true, "vuv": true, "xaf": true, "xof": true, "xpf": true,
}

// MinorFactor: pengali dari satuan utama ke satuan terkecil Stripe (IDR dan USD = 100, JPY = 1).
func MinorFactor(currency string) int64 {
	if zeroDecimal[strings.ToLower(currency)] {
		return 1
	}
	return 100
}

// majorUnits mengubah satuan terkecil Stripe ke desimal teks satuan utama, tanpa floating point.
func majorUnits(minor int64, currency string) (string, error) {
	if minor < 0 {
		return "", fmt.Errorf("negative amount %d", minor)
	}
	f := MinorFactor(currency)
	if f == 1 {
		return strconv.FormatInt(minor, 10) + ".00", nil
	}
	return fmt.Sprintf("%d.%02d", minor/f, minor%f), nil
}
