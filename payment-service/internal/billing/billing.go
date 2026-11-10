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
	// UserID: id user yang membayar. Dikirim balik di webhook sebagai referensi.
	UserID int64
	// CustomerEmail: email yang terisi di formulir Stripe.
	CustomerEmail string
	// SuccessURL: halaman tujuan setelah pembayaran berhasil.
	SuccessURL string
	// CancelURL: halaman tujuan bila user membatalkan pembayaran.
	CancelURL string
}

// Checkout: sesi pembayaran yang sudah dibuka di Stripe.
type Checkout struct {
	// URL: alamat halaman pembayaran Stripe yang dituju browser.
	URL string
	// ExpiresAt: waktu sesi ini tidak bisa dipakai lagi.
	ExpiresAt time.Time
}

// Session: bagian sesi Checkout yang kita butuhkan dari webhook.
type Session struct {
	// ID: id sesi Checkout di Stripe.
	ID string
	// UserID: dari client_reference_id; 0 bila tidak ada atau tidak valid.
	UserID int64
	// PaymentStatus: status pembayaran sesi. "paid" berarti lunas.
	PaymentStatus string
	// AmountTotal: jumlah yang dibayar dalam satuan terkecil mata uang.
	AmountTotal int64
	// Currency: kode mata uang ISO 4217 dalam huruf kecil.
	Currency string
}

// Event: satu event webhook Stripe yang sudah diverifikasi dan diurai.
type Event struct {
	// ID: id event di Stripe. Dipakai untuk mencegah pemrosesan ganda.
	ID string
	// Type: jenis event, mis. checkout.session.completed.
	Type string
	// Session: data sesi Checkout yang dibawa event ini. Hanya terisi untuk event sesi.
	Session Session
}

// Provider: sisi Stripe.
type Provider interface {
	// CreateCheckout membuka sesi pembayaran di Stripe.
	CreateCheckout(ctx context.Context, in CheckoutInput) (Checkout, error)
	// ParseEvent memverifikasi tanda tangan webhook lalu mengurai event-nya.
	ParseEvent(payload []byte, sigHeader string) (Event, error)
}

// Credit: satu pembayaran yang harus mengubah langganan.
type Credit struct {
	// EventID: id event Stripe yang membawa pembayaran ini. Dipakai untuk idempotensi.
	EventID string
	// EventType: jenis event, untuk dicatat.
	EventType string
	// SessionID: id sesi Checkout. Satu sesi hanya boleh memperpanjang sekali.
	SessionID string
	// UserID: user yang langganannya diperpanjang.
	UserID int64
	// Amount: jumlah dalam satuan utama, teks desimal (mis. "50000.00").
	Amount string
	// Currency: kode mata uang dalam huruf besar.
	Currency string
}

// Store: sisi database.
type Store interface {
	// Ping memeriksa koneksi database.
	Ping(ctx context.Context) error
	// UserEmail mengembalikan ErrUnknownUser bila user tidak ada. User yang belum owner sah (calon owner).
	UserEmail(ctx context.Context, userID int64) (string, error)
	// Credit memperpanjang langganan satu bulan (membuat baris owners bila user belum owner) dan mencatat
	// pembayaran, atomik dan idempoten per event dan per sesi. extended=false bila event/sesi sudah diproses.
	Credit(ctx context.Context, c Credit) (extended bool, expiresAt time.Time, err error)
	// RecordEvent mencatat event yang tidak mengubah langganan (supaya kiriman ulang diabaikan).
	RecordEvent(ctx context.Context, eventID, eventType string) error
}

// Service: lapisan bisnis pembayaran. Membuka checkout dan memproses webhook.
type Service struct {
	// store: akses database untuk langganan dan pembayaran.
	store Store
	// provider: sisi Stripe. Nil = pembayaran dimatikan (kecuali simulator dev).
	provider Provider
	// sim: non-nil = pembayaran PALSU untuk dev.
	sim *simulator
	// log: pencatat peristiwa.
	log *slog.Logger
	// publicBase: origin publik tanpa garis miring di akhir, dasar tautan kembali dari Stripe.
	publicBase string
}

// New membuat Service dengan store, penyedia pembayaran (boleh nil), origin publik, dan logger.
func New(store Store, provider Provider, publicOrigin string, log *slog.Logger) *Service {
	// Buang garis miring di akhir origin supaya tautan tidak berisi "//".
	return &Service{store: store, provider: provider, publicBase: strings.TrimRight(publicOrigin, "/"), log: log}
}

// Enabled melaporkan apakah pembayaran aktif: ada penyedia Stripe atau simulator dev.
func (s *Service) Enabled() bool { return s.provider != nil || s.sim != nil }

// simulator: pembayaran palsu untuk dev. Dipasang hanya bila DEV_PAYMENT_SIMULATOR=true (config menolak di produksi).
type simulator struct {
	// amount: jumlah palsu dalam satuan utama, teks desimal.
	amount string
	// currency: kode mata uang huruf besar.
	currency string
}

// WithSimulator mengganti Stripe dengan pembayaran palsu: StartCheckout langsung memperpanjang/membuat owner lewat
// jalur Credit yang sama dengan webhook asli, tanpa jaringan. HANYA dev.
func (s *Service) WithSimulator(amountMajor int64, currency string) *Service {
	// Ubah jumlah utama ke satuan terkecil lalu kembali ke teks utama, supaya formatnya sama dengan webhook asli.
	amount, _ := majorUnits(amountMajor*MinorFactor(currency), currency)
	// Pasang simulator dengan jumlah dan mata uang palsu.
	s.sim = &simulator{amount: amount, currency: strings.ToUpper(currency)}
	return s
}

// Health memeriksa koneksi database dengan batas waktu 2 detik.
func (s *Service) Health(ctx context.Context) error {
	// Batasi pemeriksaan dengan 2 detik.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// Kembalikan hasil ping database.
	return s.store.Ping(ctx)
}

// StartCheckout membuka sesi pembayaran 1 bulan untuk user: perpanjangan, atau pembayaran pertama = jadi owner.
func (s *Service) StartCheckout(ctx context.Context, userID int64) (Checkout, error) {
	// Simulator aktif: langsung catat pembayaran palsu tanpa menghubungi Stripe.
	if s.sim != nil {
		return s.simulatePayment(ctx, userID)
	}
	// Tidak ada penyedia: pembayaran dimatikan.
	if s.provider == nil {
		return Checkout{}, ErrDisabled
	}
	// Ambil email user untuk diisi di formulir Stripe. Galat (mis. user tidak ada) dikembalikan apa adanya.
	email, err := s.store.UserEmail(ctx, userID)
	if err != nil {
		return Checkout{}, err
	}
	// Buka sesi di Stripe dengan tautan kembali sukses dan batal.
	co, err := s.provider.CreateCheckout(ctx, CheckoutInput{
		UserID:        userID,
		CustomerEmail: email,
		SuccessURL:    s.publicBase + "/subscription?payment=success",
		CancelURL:     s.publicBase + "/subscription?payment=cancel",
	})
	// Gagal membuka sesi: bungkus galatnya dengan konteks.
	if err != nil {
		return Checkout{}, fmt.Errorf("create checkout: %w", err)
	}
	// Catat bahwa sesi berhasil dibuat.
	s.log.Info("checkout created", "user_id", userID)
	return co, nil
}

// simulatePayment mencatat pembayaran palsu seketika dan mengembalikan URL "berhasil" seperti redirect Stripe.
func (s *Service) simulatePayment(ctx context.Context, userID int64) (Checkout, error) {
	// Buat 8 byte acak untuk membedakan id event dan sesi palsu.
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Checkout{}, err
	}
	id := hex.EncodeToString(b[:])
	// Catat pembayaran palsu lewat jalur Credit yang sama dengan webhook asli.
	_, expires, err := s.store.Credit(ctx, Credit{
		EventID: "sim_evt_" + id, EventType: "simulated.payment", SessionID: "sim_cs_" + id,
		UserID: userID, Amount: s.sim.amount, Currency: s.sim.currency,
	})
	if err != nil {
		return Checkout{}, err
	}
	// Beri peringatan keras di log bahwa pembayaran ini palsu.
	s.log.Warn("SIMULATED payment credited (dev only, no real money)", "user_id", userID,
		"expires_at", expires.UTC().Format(time.RFC3339))
	// Kembalikan URL sukses dengan masa berlaku 10 menit, seperti sesi Checkout sungguhan.
	return Checkout{URL: s.publicBase + "/subscription?payment=success", ExpiresAt: time.Now().Add(10 * time.Minute)}, nil
}

// HandleWebhook memverifikasi tanda tangan lalu memperpanjang langganan bila pembayaran berhasil. Sumber
// kebenaran adalah webhook, bukan redirect browser. Mengembalikan error (Stripe akan mengirim ulang) hanya
// bila penyimpanan gagal; event yang tidak relevan atau user yang hilang tidak dicoba ulang.
func (s *Service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	// Penyedia tidak ada: webhook tidak bisa diproses.
	if s.provider == nil {
		return ErrDisabled
	}
	// Verifikasi tanda tangan dan urai event. Galat dibungkus sebagai tanda tangan tidak sah.
	ev, err := s.provider.ParseEvent(payload, sigHeader)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadSignature, err)
	}

	// Pilih penanganan sesuai jenis event.
	switch ev.Type {
	// Sesi selesai, atau pembayaran tertunda akhirnya berhasil: keduanya bisa membawa pembayaran.
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		// completed juga terkirim untuk metode bayar tertunda (status unpaid); perpanjang hanya saat lunas.
		if ev.Session.PaymentStatus != "paid" {
			return s.store.RecordEvent(ctx, ev.ID, ev.Type)
		}
		// Sesi lunas tanpa referensi user yang valid: tidak bisa dikaitkan ke akun, catat galat dan lewati.
		if ev.Session.UserID <= 0 {
			s.log.Error("paid session without user reference", "event_id", ev.ID, "session_id", ev.Session.ID)
			return nil
		}
		// Ubah jumlah dari satuan terkecil ke teks satuan utama. Jumlah tidak masuk akal: catat dan lewati.
		amount, err := majorUnits(ev.Session.AmountTotal, ev.Session.Currency)
		if err != nil {
			s.log.Error("bad amount on paid session", "event_id", ev.ID, "session_id", ev.Session.ID, "err", err)
			return nil
		}
		// Perpanjang langganan dan catat pembayaran. Hasilnya menentukan apa yang dicatat.
		extended, expires, err := s.store.Credit(ctx, Credit{
			EventID:   ev.ID,
			EventType: ev.Type,
			SessionID: ev.Session.ID,
			UserID:    ev.Session.UserID,
			Amount:    amount,
			Currency:  strings.ToUpper(ev.Session.Currency),
		})
		// Pilih tindakan sesuai hasil perpanjangan.
		switch {
		// User tidak ditemukan padahal uang sudah masuk: butuh tindakan manusia, mengulang tidak menolong.
		case errors.Is(err, ErrUnknownUser):
			// Uang masuk tetapi user tidak ada: butuh tindakan manusia, mengulang tidak menolong.
			s.log.Error("PAID SESSION FOR UNKNOWN USER, refund manually", "event_id", ev.ID,
				"session_id", ev.Session.ID, "user_id", ev.Session.UserID)
			return nil
		// Galat penyimpanan lain: kembalikan agar Stripe mengirim ulang.
		case err != nil:
			return err
		// Langganan diperpanjang: catat tanggal berakhir barunya.
		case extended:
			s.log.Info("subscription extended", "user_id", ev.Session.UserID,
				"session_id", ev.Session.ID, "expires_at", expires.UTC().Format(time.RFC3339))
		// Event atau sesi sudah pernah diproses: kiriman ulang, diabaikan.
		default:
			s.log.Info("duplicate payment event ignored", "event_id", ev.ID, "session_id", ev.Session.ID)
		}
		return nil
	// Event lain (expired, async_payment_failed, dll.) tidak mengubah langganan; cukup dicatat.
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

// MinorFactor mengembalikan pengali dari satuan utama ke satuan terkecil Stripe: 1 untuk mata uang tanpa
// pecahan (mis. JPY), dan 100 untuk sisanya (mis. IDR dan USD).
func MinorFactor(currency string) int64 {
	// Mata uang tanpa pecahan: satuan utama sama dengan satuan terkecil.
	if zeroDecimal[strings.ToLower(currency)] {
		return 1
	}
	return 100
}

// majorUnits mengubah satuan terkecil Stripe ke desimal teks satuan utama, tanpa floating point.
func majorUnits(minor int64, currency string) (string, error) {
	// Jumlah negatif tidak masuk akal untuk pembayaran.
	if minor < 0 {
		return "", fmt.Errorf("negative amount %d", minor)
	}
	// Ambil pengali mata uang.
	f := MinorFactor(currency)
	// Tanpa pecahan: tambahkan ".00" di belakang.
	if f == 1 {
		return strconv.FormatInt(minor, 10) + ".00", nil
	}
	// Bagi dengan pengali: bagian bulat dan dua digit pecahan.
	return fmt.Sprintf("%d.%02d", minor/f, minor%f), nil
}
