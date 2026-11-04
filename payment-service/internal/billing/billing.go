// Package billing: satu-satunya kemampuan payment-service adalah memperpanjang langganan SATU BULAN setelah
// Stripe mengonfirmasi pembayaran. Pembayaran pertama sebuah user sekaligus menjadikannya owner (membuat baris
// owners). Tidak ada plan lain, tidak ada penagihan berulang.
package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrUnknownUser: user dengan id itu tidak ada (mis. akun sudah dihapus).
	ErrUnknownUser = errors.New("unknown user")
	// ErrDisabled: kunci Stripe belum dikonfigurasi; pembayaran dimatikan.
	ErrDisabled = errors.New("payments are not configured")
	// ErrBadSignature: tanda tangan webhook tidak sah.
	ErrBadSignature = errors.New("invalid webhook signature")
)

// CheckoutInput: sesi Checkout yang akan dibuka untuk satu user (owner, atau calon owner).
type CheckoutInput struct {
	UserID        int64
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
	UserID        int64 // dari client_reference_id; 0 bila tidak ada/tidak valid
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
	UserID    int64
	Amount    string // satuan utama, desimal teks (mis. "50000.00")
	Currency  string // huruf besar
}

// Store: sisi database.
type Store interface {
	Ping(ctx context.Context) error
	// UserEmail mengembalikan ErrUnknownUser bila user tidak ada. User yang belum owner sah (calon owner).
	UserEmail(ctx context.Context, userID int64) (string, error)
	// Credit memperpanjang langganan satu bulan (membuat baris owners bila user belum owner) dan mencatat
	// pembayaran, atomik dan idempoten per event dan per sesi. extended=false bila event/sesi sudah diproses.
	Credit(ctx context.Context, c Credit) (extended bool, expiresAt time.Time, err error)
	// RecordEvent mencatat event yang tidak mengubah langganan (supaya kiriman ulang diabaikan).
	RecordEvent(ctx context.Context, eventID, eventType string) error
}

type Service struct {
	store      Store
	provider   Provider   // nil = pembayaran dimatikan (kecuali simulator dev)
	sim        *simulator // non-nil = pembayaran PALSU untuk dev
	log        *slog.Logger
	publicBase string
}

func New(store Store, provider Provider, publicOrigin string, log *slog.Logger) *Service {
	return &Service{store: store, provider: provider, publicBase: strings.TrimRight(publicOrigin, "/"), log: log}
}

func (s *Service) Enabled() bool { return s.provider != nil || s.sim != nil }

// simulator: pembayaran palsu untuk dev. Dipasang hanya bila DEV_PAYMENT_SIMULATOR=true (config menolak di produksi).
type simulator struct {
	amount   string // satuan utama, desimal teks
	currency string // huruf besar
}

// WithSimulator mengganti Stripe dengan pembayaran palsu: StartCheckout langsung memperpanjang/membuat owner lewat
// jalur Credit yang sama dengan webhook asli, tanpa jaringan. HANYA dev.
func (s *Service) WithSimulator(amountMajor int64, currency string) *Service {
	amount, _ := majorUnits(amountMajor*MinorFactor(currency), currency)
	s.sim = &simulator{amount: amount, currency: strings.ToUpper(currency)}
	return s
}

func (s *Service) Health(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.store.Ping(ctx)
}

// StartCheckout membuka sesi pembayaran 1 bulan untuk user: perpanjangan, atau pembayaran pertama = jadi owner.
func (s *Service) StartCheckout(ctx context.Context, userID int64) (Checkout, error) {
	if s.sim != nil {
		return s.simulatePayment(ctx, userID)
	}
	if s.provider == nil {
		return Checkout{}, ErrDisabled
	}
	email, err := s.store.UserEmail(ctx, userID)
	if err != nil {
		return Checkout{}, err
	}
	co, err := s.provider.CreateCheckout(ctx, CheckoutInput{
		UserID:        userID,
		CustomerEmail: email,
		SuccessURL:    s.publicBase + "/subscription?payment=success",
		CancelURL:     s.publicBase + "/subscription?payment=cancel",
	})
	if err != nil {
		return Checkout{}, fmt.Errorf("create checkout: %w", err)
	}
	s.log.Info("checkout created", "user_id", userID)
	return co, nil
}

// simulatePayment mencatat pembayaran palsu seketika dan mengembalikan URL "berhasil" seperti redirect Stripe.
func (s *Service) simulatePayment(ctx context.Context, userID int64) (Checkout, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Checkout{}, err
	}
	id := hex.EncodeToString(b[:])
	_, expires, err := s.store.Credit(ctx, Credit{
		EventID: "sim_evt_" + id, EventType: "simulated.payment", SessionID: "sim_cs_" + id,
		UserID: userID, Amount: s.sim.amount, Currency: s.sim.currency,
	})
	if err != nil {
		return Checkout{}, err
	}
	s.log.Warn("SIMULATED payment credited (dev only, no real money)", "user_id", userID,
		"expires_at", expires.UTC().Format(time.RFC3339))
	return Checkout{URL: s.publicBase + "/subscription?payment=success", ExpiresAt: time.Now().Add(10 * time.Minute)}, nil
}

// HandleWebhook memverifikasi tanda tangan lalu memperpanjang langganan bila pembayaran berhasil. Sumber
// kebenaran adalah webhook, bukan redirect browser. Mengembalikan error (Stripe akan mengirim ulang) hanya
// bila penyimpanan gagal; event yang tidak relevan atau user yang hilang tidak dicoba ulang.
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
		if ev.Session.UserID <= 0 {
			s.log.Error("paid session without user reference", "event_id", ev.ID, "session_id", ev.Session.ID)
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
			UserID:    ev.Session.UserID,
			Amount:    amount,
			Currency:  strings.ToUpper(ev.Session.Currency),
		})
		switch {
		case errors.Is(err, ErrUnknownUser):
			// Uang masuk tetapi user tidak ada: butuh tindakan manusia, mengulang tidak menolong.
			s.log.Error("PAID SESSION FOR UNKNOWN USER, refund manually", "event_id", ev.ID,
				"session_id", ev.Session.ID, "user_id", ev.Session.UserID)
			return nil
		case err != nil:
			return err
		case extended:
			s.log.Info("subscription extended", "user_id", ev.Session.UserID,
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
