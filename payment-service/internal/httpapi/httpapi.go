package httpapi

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"

	"qms/payment-service/internal/billing"
)

// maxWebhookBody membatasi payload webhook yang dibaca.
const maxWebhookBody = 1 << 20 // 1 MiB

// Billing: yang dibutuhkan API dari lapisan bisnis (dan yang bisa diganti di test).
type Billing interface {
	// Health memeriksa koneksi ke database.
	Health(ctx context.Context) error
	// StartCheckout membuka sesi pembayaran untuk user.
	StartCheckout(ctx context.Context, userID int64) (billing.Checkout, error)
	// HandleWebhook memproses payload webhook setelah tanda tangannya diperiksa.
	HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error
}

// API: handler HTTP untuk health check, checkout, dan webhook Stripe.
type API struct {
	// svc: lapisan bisnis pembayaran.
	svc Billing
	// pub: kunci publik RSA untuk memverifikasi JWT login RS256 (diterbitkan auth-service).
	pub *rsa.PublicKey
	// legacy: secret HS256 lama, hanya masa transisi. Kosong = token HS256 ditolak.
	legacy []byte
	// log: pencatat peristiwa dan galat.
	log *slog.Logger
}

// New membuat API dengan service billing, kunci publik JWT, secret HS256 lama (boleh kosong), dan logger.
func New(svc Billing, pub *rsa.PublicKey, legacySecret string, log *slog.Logger) *API {
	a := &API{svc: svc, pub: pub, log: log}
	// Secret lama hanya dipakai bila diisi; kosong berarti token HS256 ditolak.
	if legacySecret != "" {
		a.legacy = []byte(legacySecret)
	}
	return a
}

// Handler membangun router HTTP untuk ketiga endpoint.
func (a *API) Handler() http.Handler {
	// Router standar Go dengan pola metode dan path.
	mux := http.NewServeMux()
	// Health check.
	mux.HandleFunc("GET /health", a.health)
	// Pembukaan sesi pembayaran (wajib login).
	mux.HandleFunc("POST /api/v1/payments/checkout", a.checkout)
	// Webhook dari Stripe (diverifikasi tanda tangannya di service).
	mux.HandleFunc("POST /api/v1/payments/webhooks/stripe", a.webhook)
	return mux
}

// health menjawab 200 bila database terjangkau, dan 503 bila tidak.
func (a *API) health(w http.ResponseWriter, r *http.Request) {
	// Database tidak terjangkau: balas 503 dengan status unavailable.
	if err := a.svc.Health(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	// Database terjangkau: balas 200 dengan status ok.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// checkout membuka sesi pembayaran untuk user yang login, lalu mengembalikan URL checkout dan waktu kedaluwarsanya.
func (a *API) checkout(w http.ResponseWriter, r *http.Request) {
	// Verifikasi login. Gagal berarti token tidak ada, salah, atau tidak sah.
	userID, ok := a.authenticate(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized user")
		return
	}
	// Batasi pembukaan sesi checkout dengan 15 detik.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Minta sesi checkout ke service, lalu petakan galat ke kode HTTP.
	co, err := a.svc.StartCheckout(ctx, userID)
	switch {
	// User tidak ditemukan: 403, karena akun yang login tidak dikenal.
	case errors.Is(err, billing.ErrUnknownUser):
		writeError(w, http.StatusForbidden, "UNKNOWN_USER", "account not found")
	// Pembayaran belum dikonfigurasi: 503.
	case errors.Is(err, billing.ErrDisabled):
		writeError(w, http.StatusServiceUnavailable, "PAYMENT_UNAVAILABLE", "payments are temporarily unavailable")
	// Galat lain dari penyedia atau database: catat, lalu balas 502 tanpa detail.
	case err != nil:
		a.log.Error("start checkout failed", "user_id", userID, "err", err)
		writeError(w, http.StatusBadGateway, "PAYMENT_PROVIDER_ERROR", "payment provider error, try again later")
	// Sukses: kirim URL checkout dan waktu kedaluwarsanya dalam format RFC 3339.
	default:
		writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{
			"checkout_url": co.URL,
			"expires_at":   co.ExpiresAt.Format(time.RFC3339),
		}})
	}
}

// webhook menerima payload Stripe, meneruskannya ke service untuk diverifikasi dan diproses, lalu memetakan hasilnya.
func (a *API) webhook(w http.ResponseWriter, r *http.Request) {
	// Baca body dengan batas 1 MiB. Gagal dibaca (termasuk melewati batas) dibalas 400.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "could not read body")
		return
	}
	// Batasi pemrosesan webhook dengan 15 detik.
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Teruskan body dan header tanda tangan ke service, lalu petakan galatnya.
	err = a.svc.HandleWebhook(ctx, body, r.Header.Get("Stripe-Signature"))
	switch {
	// Tanda tangan tidak sah: catat sebagai peringatan dan balas 400.
	case errors.Is(err, billing.ErrBadSignature):
		a.log.Warn("stripe webhook signature invalid", "remote", r.RemoteAddr, "err", err)
		writeError(w, http.StatusBadRequest, "BAD_SIGNATURE", "invalid webhook signature")
	// Pembayaran belum dikonfigurasi: 503.
	case errors.Is(err, billing.ErrDisabled):
		writeError(w, http.StatusServiceUnavailable, "PAYMENT_UNAVAILABLE", "payments are not configured")
	// Galat penyimpanan: balas 500 supaya Stripe mengirim ulang; perpanjangan terkejar saat penyimpanan pulih.
	case err != nil:
		// 5xx membuat Stripe mengirim ulang; perpanjangan terkejar saat penyimpanan pulih.
		a.log.Error("webhook handling failed", "err", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "internal server error")
	// Berhasil diterima: balas 200 dengan received true.
	default:
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"received": true}})
	}
}

// authenticate memverifikasi JWT QMS secara lokal (RS256 dengan kunci publik; HS256 hanya dengan secret transisi)
// dan mengembalikan id user. Tidak menghubungi auth-service.
func (a *API) authenticate(r *http.Request) (int64, bool) {
	// Ambil header Authorization dan pastikan berbentuk "Bearer <token>".
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return 0, false
	}
	// Parse dan verifikasi token. Algoritma ditegakkan per kunci: RS256 hanya dengan kunci publik, HS256 hanya
	// dengan secret lama. Ini mencegah algorithm confusion dan menolak alg=none.
	tok, err := jwt.Parse(strings.TrimPrefix(h, "Bearer "), func(t *jwt.Token) (any, error) {
		switch t.Method {
		case jwt.SigningMethodRS256:
			return a.pub, nil
		case jwt.SigningMethodHS256:
			// HS256 hanya sah selama masa transisi (secret lama diisi).
			if len(a.legacy) > 0 {
				return a.legacy, nil
			}
		}
		return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
	})
	// Parse gagal atau token tidak valid (kedaluwarsa, tanda tangan salah): tolak.
	if err != nil || !tok.Valid {
		return 0, false
	}
	// Ambil klaim dalam bentuk peta; bila bukan, tolak.
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	// Klaim sub harus berupa teks.
	sub, ok := claims["sub"].(string)
	if !ok {
		return 0, false
	}
	// Ubah sub ke id user. Bukan angka atau tidak positif: tolak.
	id, err := strconv.ParseInt(sub, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}
